package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/collector"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/sourceschema"
	"github.com/knadh/koanf/v2"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

var cronStandardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const maxReportErrLen = 2048

func main() {
	collector.Main(collector.MainConfig{
		ID:            "rss",
		DefaultListen: ":8087",
		ConfigYAML:    "collector-rss.yaml",
		Work:          rssWork,
	})
}

//gocyclo:ignore
func rssWork(
	ctx context.Context,
	k *koanf.Koanf,
	log *logrus.Logger,
	_ *bootstrap.WorkerRuntime,
	amqpClient *amqpctl.Client,
	controllerBaseURL string,
) error {
	if amqpClient == nil {
		return errors.New("nil AMQP client")
	}
	if err := sourceschema.Publish(ctx, amqpClient, sourceschema.RssV1()); err != nil {
		log.WithError(err).Warn("publish SourceSchema failed")
	}
	pollSec := collector.DefaultPollIntervalSeconds
	if k != nil && k.Exists("poll_interval_seconds") {
		if v := k.Int("poll_interval_seconds"); v > 0 {
			pollSec = v
		}
	}
	fetchTimeout := 90 * time.Second
	if k != nil && k.Exists("fetch_timeout_seconds") {
		if v := k.Int("fetch_timeout_seconds"); v > 0 {
			fetchTimeout = time.Duration(v) * time.Second
		}
	}
	userAgent := ""
	if k != nil && k.Exists("user_agent") {
		userAgent = strings.TrimSpace(k.String("user_agent"))
	}
	itemsMax := 25
	if k != nil && k.Exists("items_max_per_feed") {
		if v := k.Int("items_max_per_feed"); v > 0 {
			itemsMax = v
		}
	}

	startCollectionRequestConsumer(ctx, log, amqpClient, controllerBaseURL, fetchTimeout, userAgent, itemsMax)

	ticker := time.NewTicker(time.Duration(pollSec) * time.Second)
	defer ticker.Stop()

	for {
		if err := runRssPoll(ctx, log, k, amqpClient, controllerBaseURL, fetchTimeout, userAgent, itemsMax); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Warn("rss collector poll tick failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

//gocyclo:ignore
func runRssPoll(
	ctx context.Context,
	log *logrus.Logger,
	k *koanf.Koanf,
	amqpClient *amqpctl.Client,
	controllerBaseURL string,
	fetchTimeout time.Duration,
	userAgent string,
	itemsMax int,
) error {
	_ = k
	sources, err := listRssCollectionSources(ctx, controllerBaseURL)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, src := range sources {
		if src == nil || !src.GetEnabled() {
			continue
		}
		if collectionSourceRunNowOnly(src) {
			continue
		}
		if !collectionSourceDue(src, now) {
			continue
		}
		runOneRssSource(ctx, log, amqpClient, controllerBaseURL, src, now, fetchTimeout, userAgent, itemsMax)
	}
	return nil
}

func collectionSourceRunNowOnly(src *icehivev1.CollectionSource) bool {
	return strings.TrimSpace(src.GetCronLine()) == ""
}

func collectionSourceDue(src *icehivev1.CollectionSource, now time.Time) bool {
	if collectionSourceRunNowOnly(src) {
		return false
	}
	nd := src.GetNextDueUnixMs()
	if nd <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(nd).UTC())
}

func nextDueForReport(sched cron.Schedule, after time.Time, runNowOnly bool) int64 {
	if runNowOnly {
		return 0
	}
	return sched.Next(after).UnixMilli()
}

//gocyclo:ignore
func runOneRssSource(
	ctx context.Context,
	log *logrus.Logger,
	amqpClient *amqpctl.Client,
	controllerBaseURL string,
	src *icehivev1.CollectionSource,
	now time.Time,
	fetchTimeout time.Duration,
	userAgent string,
	itemsMax int,
) {
	cronLine := strings.TrimSpace(src.GetCronLine())
	runNowOnly := cronLine == ""
	var sched cron.Schedule
	if !runNowOnly {
		var err error
		sched, err = cronStandardParser.Parse(cronLine)
		if err != nil {
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), 0)
			log.WithError(err).WithField("source_id", src.GetId()).Error("invalid cron_line on collection source")
			return
		}
	}
	nextDue := nextDueForReport(sched, now, runNowOnly)

	spec, err := parseRSSSourceSpec(src.GetSourceSpec())
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithField("source_id", src.GetId()).Warn("rss source_spec parse failed")
		return
	}
	feedURL := spec.FeedURL
	maxArticles := effectiveArticlesMax(spec, itemsMax)

	fp := newFeedParser(fetchTimeout, userAgent)
	feed, err := fetchFeed(ctx, fp, feedURL)
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithFields(logrus.Fields{
			"source_id": src.GetId(), "feed_url": feedURL,
		}).Warn("rss feed fetch or parse failed")
		return
	}

	items := append([]*gofeed.Item(nil), feed.Items...)
	sortFeedItemsNewestFirst(items)
	if maxArticles > 0 && len(items) > maxArticles {
		items = items[:maxArticles]
	}

	feedTitle := strings.TrimSpace(feed.Title)
	feedType := strings.TrimSpace(feed.FeedType)
	if feedType == "" {
		feedType = "unknown"
	}

	feedEntity, ok := buildRssFeedEntity(src.GetId(), feedURL, feed, feedType, len(items))
	if !ok {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, "rss: could not build RssFeed entity", nextDue)
		log.WithField("source_id", src.GetId()).Warn("build RssFeed entity failed")
		return
	}
	feedBody, err := json.Marshal(feedEntity)
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithField("source_id", src.GetId()).Error("RssFeed entity json marshal failed")
		return
	}
	if err := amqpClient.PublishJSON(ctx, amqpctl.RoutingKeyCollectorEntities, feedBody); err != nil {
		after := time.Now().UTC()
		nextDue = nextDueForReport(sched, after, runNowOnly)
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithField("source_id", src.GetId()).Warn("publish RssFeed entity failed")
		return
	}

	published := 0
	for _, it := range items {
		entity, ok := buildRssArticleEntity(src.GetId(), feedURL, feedTitle, feedType, it)
		if !ok {
			continue
		}
		body, err := json.Marshal(entity)
		if err != nil {
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Error("RssArticle entity json marshal failed")
			return
		}
		if err := amqpClient.PublishJSON(ctx, amqpctl.RoutingKeyCollectorEntities, body); err != nil {
			after := time.Now().UTC()
			nextDue = nextDueForReport(sched, after, runNowOnly)
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Warn("publish RssArticle entity failed")
			return
		}
		published++
	}

	after := time.Now().UTC()
	nextDue = nextDueForReport(sched, after, runNowOnly)
	if err := reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), true, "", nextDue); err != nil {
		log.WithError(err).WithField("source_id", src.GetId()).Warn("report collection source run failed")
		return
	}
	log.WithFields(logrus.Fields{
		"source_id": src.GetId(), "feed_url": feedURL,
		"entry_count": len(items), "published_articles": published,
	}).Info("published RssFeed and RssArticle entities")
}

func truncateErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= maxReportErrLen {
		return s
	}
	return s[:maxReportErrLen]
}
