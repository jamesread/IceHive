package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/collector"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/sourceschema"
	"github.com/knadh/koanf/v2"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var cronStandardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const maxReportErrLen = 2048

func main() {
	collector.Main(collector.MainConfig{
		ID:            "github",
		DefaultListen: ":8081",
		ConfigYAML:    "collector-github.yaml",
		Work:          githubWork,
	})
}

func githubWork(
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
	issueColl := newIssueCollector(k)
	if err := sourceschema.Publish(ctx, amqpClient, sourceschema.GitHubV1()); err != nil {
		log.WithError(err).Warn("publish SourceSchema failed")
	}
	pollSec := collector.DefaultPollIntervalSeconds
	if k != nil && k.Exists("poll_interval_seconds") {
		if v := k.Int("poll_interval_seconds"); v > 0 {
			pollSec = v
		}
	}
	startCollectionRequestConsumer(ctx, log, amqpClient, controllerBaseURL, issueColl)

	ticker := time.NewTicker(time.Duration(pollSec) * time.Second)
	defer ticker.Stop()

	for {
		if err := runGithubPoll(ctx, log, amqpClient, controllerBaseURL, issueColl); err != nil && !errors.Is(err, context.Canceled) {
			log.WithError(err).Warn("github collector poll tick failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func runGithubPoll(ctx context.Context, log *logrus.Logger, amqpClient *amqpctl.Client, controllerBaseURL string, issueColl *issueCollector) error {
	token, err := getGitHubToken(ctx, controllerBaseURL)
	if err != nil {
		return err
	}
	ghClient := newGitHubHTTPClient(ctx, token)

	sources, err := listGithubCollectionSources(ctx, controllerBaseURL)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, src := range sources {
		if src == nil || !src.GetEnabled() {
			continue
		}
		if !collectionSourceDue(src, now) {
			continue
		}
		runOneSource(ctx, log, amqpClient, controllerBaseURL, ghClient, src, now, issueColl)
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

func newGitHubHTTPClient(ctx context.Context, token string) *github.Client {
	token = strings.TrimSpace(token)
	if token == "" {
		return github.NewClient(nil)
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func runOneSource(
	ctx context.Context,
	log *logrus.Logger,
	amqpClient *amqpctl.Client,
	controllerBaseURL string,
	ghClient *github.Client,
	src *icehivev1.CollectionSource,
	now time.Time,
	issueColl *issueCollector,
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

	owner, repoName, allUnderLogin, srcOpts, err := parseGitHubSourceSpec(src.GetSourceSpec())
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		log.WithError(err).WithField("source_id", src.GetId()).Warn("source_spec parse failed")
		return
	}
	log.WithFields(logrus.Fields{
		"source_id":       src.GetId(),
		"source_spec":     src.GetSourceSpec(),
		"opt_dependabot":  srcOpts.Dependabot,
		"opt_prs":         srcOpts.PRs,
		"opt_issues":      srcOpts.Issues,
		"all_under_login": allUnderLogin,
		"resolved_owner":  owner,
		"resolved_repo":   repoName,
	}).Debug("github source_spec parsed for collection run")

	repos, err := fetchReposForSource(ctx, ghClient, owner, repoName, allUnderLogin)
	if err != nil {
		_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
		logFields := logrus.Fields{"source_id": src.GetId()}
		if allUnderLogin {
			logFields["login"] = owner
			logFields["scope"] = "org.repos"
		} else {
			logFields["repo"] = owner + "/" + repoName
		}
		log.WithError(err).WithFields(logFields).Warn("GitHub repo fetch failed")
		return
	}

	for _, repo := range repos {
		if enriched, err := enrichRepo(ctx, ghClient, repo); err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"source_id": src.GetId(),
				"repo":      repo.GetFullName(),
			}).Warn("GitHub repo metadata enrich failed (using list payload)")
		} else if enriched != nil {
			repo = enriched
		}
		var depSnap *dependabotSnapshot
		if srcOpts.Dependabot {
			if repo.GetArchived() {
				log.WithFields(logrus.Fields{
					"source_id": src.GetId(),
					"repo":      repo.GetFullName(),
				}).Info("skipping dependabot fetch for archived repository")
			} else {
				snap := fetchDependabotAlertsForRepo(ctx, ghClient, repoOwnerLogin(repo), repo.GetName())
				depSnap = &snap
				if snap.Err != "" {
					log.WithFields(logrus.Fields{
						"source_id": src.GetId(),
						"repo":      repo.GetFullName(),
						"error":     snap.Err,
					}).Warn("dependabot alerts fetch failed (entity will include error field)")
				}
			}
		}
		var prSnap *pullRequestsSnapshot
		if srcOpts.PRs {
			if repo.GetArchived() {
				log.WithFields(logrus.Fields{
					"source_id": src.GetId(),
					"repo":      repo.GetFullName(),
					"reason":    "archived_repo_pr_snap_nil",
				}).Warn("github +pr: skipping PR list for archived repo; entity omits pull_request_* / pull_requests (often shown as null downstream)")
			} else {
				snap := fetchPullRequestsForRepo(ctx, log, ghClient, repoOwnerLogin(repo), repo.GetName())
				prSnap = &snap
				if snap.Err != "" {
					log.WithFields(logrus.Fields{
						"source_id": src.GetId(),
						"repo":      repo.GetFullName(),
						"error":     snap.Err,
					}).Warn("pull requests fetch failed (entity includes pull_requests_fetch_error; counts zero)")
				} else {
					log.WithFields(logrus.Fields{
						"source_id":           src.GetId(),
						"repo":                repo.GetFullName(),
						"pr_total":            len(snap.PRs),
						"pr_open":             countOpenPullRequests(snap.PRs),
						"pr_fetch_capped":     snap.FetchCapped,
						"pr_max_in_entity":    maxPullRequestsInEntity,
						"pr_entity_truncated": len(snap.PRs) > maxPullRequestsInEntity || snap.FetchCapped,
					}).Debug("pull requests snapshot attached to entity")
				}
			}
		}
		var issueResult issuesCollectionResult
		if srcOpts.Issues {
			if repo.GetArchived() {
				log.WithFields(logrus.Fields{
					"source_id": src.GetId(),
					"repo":      repo.GetFullName(),
					"reason":    "archived_repo_issue_snap_nil",
				}).Warn("github +issue: skipping issue list for archived repo")
			} else {
				issueResult = issueColl.Collect(ctx, log, ghClient, repoOwnerLogin(repo), repo.GetName(), repo)
				if issueResult.Err != "" {
					log.WithFields(logrus.Fields{
						"source_id": src.GetId(),
						"repo":      repo.GetFullName(),
						"error":     issueResult.Err,
					}).Warn("github issues fetch failed")
				} else if issueResult.CacheHit {
					log.WithFields(logrus.Fields{
						"source_id":         src.GetId(),
						"repo":              repo.GetFullName(),
						"unchanged_skipped": issueResult.UnchangedSkipped,
					}).Info("github issues unchanged; skipped fetch and publish")
				} else {
					log.WithFields(logrus.Fields{
						"source_id":         src.GetId(),
						"repo":              repo.GetFullName(),
						"issue_fetched":     issueResult.FetchedCount,
						"issue_published":   len(issueResult.IssuesToPublish),
						"unchanged_skipped": issueResult.UnchangedSkipped,
						"issue_fetch_capped": issueResult.FetchCapped,
					}).Debug("github issues collection attached to collection run")
				}
			}
		}
		entity := buildGitRepoEntity(repo, depSnap, prSnap)
		if err := publishCollectorEntity(ctx, amqpClient, entity); err != nil {
			_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
			log.WithError(err).WithField("source_id", src.GetId()).Warn("publish GitRepo entity failed")
			return
		}
		if depSnap != nil && depSnap.Err == "" {
			for _, alert := range depSnap.Alerts {
				issueEntity, ok := buildDependabotIssueEntity(repo, alert)
				if !ok {
					continue
				}
				if err := publishCollectorEntity(ctx, amqpClient, issueEntity); err != nil {
					after := time.Now().UTC()
					nextDue = nextDueForReport(sched, after, runNowOnly)
					_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
					log.WithError(err).WithFields(logrus.Fields{
						"source_id": src.GetId(), "repo": repo.GetFullName(),
					}).Warn("publish DependabotIssue entity failed")
					return
				}
			}
		}
		if issueResult.Err == "" {
			for _, issue := range issueResult.IssuesToPublish {
				ghIssueEntity, ok := buildGitHubIssueEntity(repo, issue)
				if !ok {
					continue
				}
				if err := publishCollectorEntity(ctx, amqpClient, ghIssueEntity); err != nil {
					after := time.Now().UTC()
					nextDue = nextDueForReport(sched, after, runNowOnly)
					_ = reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), false, truncateErr(err), nextDue)
					log.WithError(err).WithFields(logrus.Fields{
						"source_id": src.GetId(), "repo": repo.GetFullName(),
					}).Warn("publish GitHubIssue entity failed")
					return
				}
			}
		}
	}

	after := time.Now().UTC()
	nextDue = nextDueForReport(sched, after, runNowOnly)
	if err := reportCollectionSourceRun(ctx, controllerBaseURL, src.GetId(), true, "", nextDue); err != nil {
		log.WithError(err).WithField("source_id", src.GetId()).Warn("report collection source run failed")
		return
	}
	logFields := logrus.Fields{
		"source_id":  src.GetId(),
		"repo_count": len(repos),
		"all_repos":  allUnderLogin,
	}
	if len(repos) == 1 {
		logFields["repo"] = repos[0].GetFullName()
	}
	log.WithFields(logFields).Info("published GitHub repo entities")
}

func publishCollectorEntity(ctx context.Context, amqpClient *amqpctl.Client, entity entityMessage) error {
	body, err := json.Marshal(entity)
	if err != nil {
		return err
	}
	return amqpClient.PublishJSON(ctx, amqpctl.RoutingKeyCollectorEntities, body)
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
