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
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

var cronStandardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const maxReportErrLen = 2048

func main() {
	collector.Main(collector.MainConfig{
		ID:            "jmap",
		DefaultListen: ":8086",
		ConfigYAML:    "collector-jmap.yaml",
		Work:          jmapWork,
	})
}

func jmapWork(
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
	if err := sourceschema.Publish(ctx, amqpClient, sourceschema.JmapV1()); err != nil {
		log.WithError(err).Warn("publish SourceSchema failed")
	}
	pollSec := collector.DefaultPollIntervalSeconds
	if k != nil && k.Exists("poll_interval_seconds") {
		if v := k.Int("poll_interval_seconds"); v > 0 {
			pollSec = v
		}
	}
	startCollectionRequestConsumer(ctx, log, amqpClient, controllerBaseURL)

	ticker := time.NewTicker(time.Duration(pollSec) * time.Second)
	defer ticker.Stop()

	for {
		if err := runJmapPoll(ctx, log, k, amqpClient, controllerBaseURL); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Warn("jmap collector poll tick failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func logJmapConnectionOK(log *logrus.Logger, rt *jmapRuntime) {
	if log == nil || rt == nil {
		return
	}
	fields := logrus.Fields{
		"jmap_account_id": rt.AccountID,
		"jmap_api_url":    rt.APIURL,
	}
	if rt.APIURLRewrittenFromSession {
		fields["jmap_api_url_rewritten_from_session"] = true
		log.WithFields(fields).Warn("JMAP connection ready: POST base was the session URL (GET-only, HTTP 405); rewritten to .../jmap/api/. Prefer ICEHIVE_JMAP_SESSION_URL for GET session only and leave ICEHIVE_JMAP_API_URL unset, or set ICEHIVE_JMAP_API_URL to the session JSON apiUrl (e.g. https://api.fastmail.com/jmap/api/)")
		return
	}
	log.WithFields(fields).Info("JMAP connection successful")
}

func runJmapPoll(ctx context.Context, log *logrus.Logger, k *koanf.Koanf, amqpClient *amqpctl.Client, controllerBaseURL string) error {
	rt, err := jmapRuntimeFromEnv(ctx)
	if err != nil {
		return err
	}
	logJmapConnectionOK(log, rt)

	sources, err := listJmapCollectionSources(ctx, controllerBaseURL)
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"collector_type": collectorJmapType,
		"sources_total":  len(sources),
	}).Info("jmap poll: listed collection sources from controller")

	if len(sources) == 0 {
		log.Info("jmap poll: no collection sources for collector-jmap (create one with collector_type collector-jmap)")
	}

	now := time.Now().UTC()
	var skippedNil, skippedDisabled, skippedRunNowOnly, skippedNotDue, ran int
	for _, src := range sources {
		if src == nil {
			skippedNil++
			log.Warn("jmap poll: nil collection source entry in list (skipped)")
			continue
		}
		if !src.GetEnabled() {
			skippedDisabled++
			log.WithFields(logrus.Fields{
				"source_id": src.GetId(),
				"enabled":   false,
			}).Info("jmap poll: skipped disabled collection source")
			continue
		}
		if collectionSourceRunNowOnly(src) {
			skippedRunNowOnly++
			log.WithFields(logrus.Fields{
				"source_id":   src.GetId(),
				"source_spec": src.GetSourceSpec(),
			}).Info("jmap poll: skipped source with empty cron_line (run-now-only; not polled on ticker — set cron_line for scheduled runs or trigger a collection request)")
			continue
		}
		if !collectionSourceDue(src, now) {
			skippedNotDue++
			log.WithFields(logrus.Fields{
				"source_id":         src.GetId(),
				"next_due_unix_ms":  src.GetNextDueUnixMs(),
				"cron_line":         src.GetCronLine(),
			}).Debug("jmap poll: skipped source not yet due")
			continue
		}
		ran++
		runOneJmapSource(ctx, log, amqpClient, controllerBaseURL, rt, src, now)
	}
	if ran == 0 && len(sources) > 0 {
		log.WithFields(logrus.Fields{
			"sources_total":          len(sources),
			"skipped_nil":          skippedNil,
			"skipped_disabled":     skippedDisabled,
			"skipped_run_now_only": skippedRunNowOnly,
			"skipped_not_due":      skippedNotDue,
		}).Info("jmap poll: no collection sources ran this tick")
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

func runOneJmapSource(
	ctx context.Context,
	log *logrus.Logger,
	amqpClient *amqpctl.Client,
	controllerBaseURL string,
	rt *jmapRuntime,
	src *icehivev1.CollectionSource,
	now time.Time,
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

	log.WithFields(logrus.Fields{
		"source_id":    src.GetId(),
		"source_spec":  src.GetSourceSpec(),
		"cron_line":    cronLine,
		"run_now_only": runNowOnly,
	}).Info("jmap: starting collection run for source")

	mailboxID, resolveInbox, err := parseJmapMailboxSourceSpec(src.GetSourceSpec())
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithField("source_id", src.GetId()).Warn("source_spec parse failed")
		return
	}
	if resolveInbox {
		log.WithField("source_id", src.GetId()).Info("jmap: resolving inbox mailbox id (Mailbox/query role=inbox)")
		mailboxID, err = rt.resolveInboxMailboxID(ctx)
		if err != nil {
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Warn("inbox mailbox resolution failed (Mailbox/query role inbox)")
			return
		}
		log.WithFields(logrus.Fields{
			"source_id":  src.GetId(),
			"mailbox_id": mailboxID,
		}).Info("jmap: resolved mailbox id for collection")
	} else {
		log.WithFields(logrus.Fields{
			"source_id":  src.GetId(),
			"mailbox_id": mailboxID,
		}).Info("jmap: using explicit mailbox id from source_spec or ICEHIVE_JMAP_MAILBOX_ID")
	}

	rows, err := rt.listEmailThreads(ctx, log, mailboxID)
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithFields(logrus.Fields{
			"source_id":  src.GetId(),
			"mailbox_id": mailboxID,
		}).Warn("JMAP thread fetch failed")
		return
	}
	if len(rows) == 0 {
		log.WithFields(logrus.Fields{
			"source_id":  src.GetId(),
			"mailbox_id": mailboxID,
		}).Info("jmap: collection run finished with zero EmailThread rows (nothing to publish; see prior jmap: Email/query / Email/get logs)")
	}

	for _, row := range rows {
		log.WithFields(logrus.Fields{
			"source_id":  src.GetId(),
			"thread_id":  row.ThreadID,
			"mailbox_id": row.MailboxID,
		}).Info("jmap: publishing EmailThread entity to AMQP")
		entity := buildEmailThreadEntity(row)
		body, err := json.Marshal(entity)
		if err != nil {
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Error("entity json marshal failed")
			return
		}
		if err := amqpClient.PublishJSON(ctx, amqpctl.RoutingKeyCollectorEntities, body); err != nil {
			after := time.Now().UTC()
			nextDue = nextDueForReport(sched, after, runNowOnly)
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Warn("publish entity failed")
			return
		}
	}

	after := time.Now().UTC()
	nextDue = nextDueForReport(sched, after, runNowOnly)
	if err := reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), true, "", nextDue); err != nil {
		log.WithError(err).WithField("source_id", src.GetId()).Warn("report collection source run failed")
		return
	}
	log.WithFields(logrus.Fields{
		"source_id":    src.GetId(),
		"mailbox_id":   mailboxID,
		"thread_count": len(rows),
	}).Info("published JMAP EmailThread entities")
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
