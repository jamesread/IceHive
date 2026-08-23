package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
)

const (
	issueCacheVersion       = 1
	issueCacheOverlap       = 2 * time.Minute
	issueCacheProbePageSize = 10
)

type issueCollector struct {
	dir     string
	enabled bool
}

type issuesCollectionResult struct {
	Err              string
	IssuesToPublish  []*github.Issue
	CacheHit         bool
	FetchCapped      bool
	FetchedCount     int
	UnchangedSkipped int
}

type repoIssueCacheFile struct {
	RepoFullName    string                      `json:"repo_full_name"`
	LatestIssueID   string                      `json:"latest_issue_id"`
	Issues          map[string]cachedIssueEntry `json:"issues"`
	LatestUpdatedAt string                      `json:"latest_updated_at"`
	Version         int                         `json:"version"`
}

type cachedIssueEntry struct {
	UpdatedAt string          `json:"updated_at"`
	Issue     json.RawMessage `json:"issue"`
}

func newIssueCollector(k *koanf.Koanf) *issueCollector {
	enabled := true
	if k != nil && k.Exists("issue_cache_enabled") {
		enabled = k.Bool("issue_cache_enabled")
	}
	return &issueCollector{
		dir:     issueCacheDir(k),
		enabled: enabled,
	}
}

func issueCacheDir(k *koanf.Koanf) string {
	if k != nil && k.Exists("issue_cache_dir") {
		if d := strings.TrimSpace(k.String("issue_cache_dir")); d != "" {
			return d
		}
	}
	if base, err := os.UserCacheDir(); err == nil {
		return filepath.Join(base, "icehive", "collector-github", "issues")
	}
	return ".issue-cache"
}

//gocyclo:ignore
func (c *issueCollector) Collect(
	ctx context.Context,
	log *logrus.Logger,
	gh *github.Client,
	sourceID, owner, repo string,
	repoObj *github.Repository,
) issuesCollectionResult {
	if !c.enabled {
		snap := fetchIssuesForRepoSince(ctx, log, gh, sourceID, owner, repo, time.Time{})
		return issuesCollectionResult{
			IssuesToPublish: snap.Issues,
			Err:             snap.Err,
			FetchCapped:     snap.FetchCapped,
			FetchedCount:    len(snap.Issues),
		}
	}

	repoUID := ""
	if repoObj != nil {
		repoUID = norm.NFC.String(repoObj.GetNodeID())
	}

	cache, err := c.load(owner, repo)
	if err != nil && log != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"owner": owner, "repo": repo, "cache_dir": c.dir,
		}).Warn("github issue cache load failed; collecting without cache")
		cache = nil
	}

	if cache != nil && len(cache.Issues) > 0 {
		probe, probeErr := probeLatestRepoIssue(ctx, sourceID, gh, owner, repo)
		if probeErr != nil {
			if log != nil {
				log.WithError(probeErr).WithFields(logrus.Fields{
					"owner": owner, "repo": repo,
				}).Debug("github issue cache probe failed; falling back to incremental fetch")
			}
		} else if issueListUnchanged(cache, probe) {
			if log != nil {
				log.WithFields(logrus.Fields{
					"owner": owner, "repo": repo,
					"cached_issues": len(cache.Issues),
				}).Debug("github issue cache hit: latest updated issue unchanged")
			}
			return issuesCollectionResult{
				UnchangedSkipped: len(cache.Issues),
				CacheHit:         true,
			}
		}
	}

	since := time.Time{}
	if cache != nil {
		if t, ok := cache.latestUpdatedTime(); ok {
			since = t.Add(-issueCacheOverlap)
		}
	}

	snap := fetchIssuesForRepoSince(ctx, log, gh, sourceID, owner, repo, since)
	if snap.Err != "" {
		return issuesCollectionResult{Err: snap.Err, FetchCapped: snap.FetchCapped}
	}

	toPublish, merged, skipped := mergeIssueCache(cache, snap.Issues, repoUID)
	if merged != nil {
		merged.RepoFullName = owner + "/" + repo
		if saveErr := c.save(owner, repo, merged); saveErr != nil && log != nil {
			log.WithError(saveErr).WithFields(logrus.Fields{
				"owner": owner, "repo": repo, "cache_dir": c.dir,
			}).Warn("github issue cache save failed")
		}
	}

	if log != nil {
		log.WithFields(logrus.Fields{
			"owner": owner, "repo": repo,
			"fetched": len(snap.Issues), "published": len(toPublish),
			"unchanged_skipped": skipped, "incremental": !since.IsZero(),
			"fetch_capped": snap.FetchCapped,
		}).Debug("github issue collection complete")
	}

	return issuesCollectionResult{
		IssuesToPublish:  toPublish,
		FetchCapped:      snap.FetchCapped,
		UnchangedSkipped: skipped,
		FetchedCount:     len(snap.Issues),
	}
}

//gocyclo:ignore
func issueListUnchanged(cache *repoIssueCacheFile, probe *github.Issue) bool {
	if cache == nil || probe == nil {
		return false
	}
	probeID := norm.NFC.String(probe.GetNodeID())
	if probeID == "" || probeID != cache.LatestIssueID {
		return false
	}
	probeUpdated := issueUpdatedAtUTC(probe)
	cacheUpdated, ok := cache.latestUpdatedTime()
	if !ok {
		return false
	}
	return probeUpdated.Equal(cacheUpdated)
}

func issueUpdatedAtUTC(issue *github.Issue) time.Time {
	if issue == nil {
		return time.Time{}
	}
	return issue.GetUpdatedAt().UTC()
}

func issueUpdatedAtRFC3339(issue *github.Issue) string {
	t := issueUpdatedAtUTC(issue)
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

//gocyclo:ignore
func mergeIssueCache(cache *repoIssueCacheFile, fetched []*github.Issue, repoUID string) ([]*github.Issue, *repoIssueCacheFile, int) {
	out := &repoIssueCacheFile{
		Version: issueCacheVersion,
		Issues:  map[string]cachedIssueEntry{},
	}
	if cache != nil && cache.Issues != nil {
		for k, v := range cache.Issues {
			out.Issues[k] = v
		}
		if cache.RepoFullName != "" {
			out.RepoFullName = cache.RepoFullName
		}
	}

	var toPublish []*github.Issue
	skipped := 0
	for _, issue := range fetched {
		if issue == nil || issue.IsPullRequest() {
			continue
		}
		id := githubIssueSourceUniqueID(repoUID, issue)
		if id == "" {
			continue
		}
		updated := issueUpdatedAtRFC3339(issue)
		if prev, ok := out.Issues[id]; ok && prev.UpdatedAt == updated {
			skipped++
			continue
		}
		raw, err := json.Marshal(issue)
		if err != nil {
			continue
		}
		out.Issues[id] = cachedIssueEntry{
			UpdatedAt: updated,
			Issue:     raw,
		}
		toPublish = append(toPublish, issue)
	}

	refreshLatestIssueMarker(out)
	return toPublish, out, skipped
}

//gocyclo:ignore
func refreshLatestIssueMarker(cache *repoIssueCacheFile) {
	if cache == nil || len(cache.Issues) == 0 {
		return
	}
	var latestID string
	var latestAt time.Time
	for id, entry := range cache.Issues {
		t, err := time.Parse(time.RFC3339, entry.UpdatedAt)
		if err != nil {
			continue
		}
		t = t.UTC()
		if latestID == "" || t.After(latestAt) {
			latestAt = t
			latestID = id
		}
	}
	cache.LatestIssueID = latestID
	if !latestAt.IsZero() {
		cache.LatestUpdatedAt = latestAt.Format(time.RFC3339)
	}
}

func (c *repoIssueCacheFile) latestUpdatedTime() (time.Time, bool) {
	if c == nil || strings.TrimSpace(c.LatestUpdatedAt) == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, c.LatestUpdatedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func (c *issueCollector) cachePath(owner, repo string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(owner + "__" + repo)
	return filepath.Join(c.dir, safe+".json")
}

func (c *issueCollector) load(owner, repo string) (*repoIssueCacheFile, error) {
	path := c.cachePath(owner, repo)
	// #nosec G304 -- cachePath derives the filename from sanitized repository identifiers.
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache repoIssueCacheFile
	if err := json.Unmarshal(b, &cache); err != nil {
		return nil, err
	}
	if cache.Issues == nil {
		cache.Issues = map[string]cachedIssueEntry{}
	}
	return &cache, nil
}

func (c *issueCollector) save(owner, repo string, cache *repoIssueCacheFile) error {
	if cache == nil {
		return errors.New("nil issue cache")
	}
	if err := os.MkdirAll(c.dir, 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	path := c.cachePath(owner, repo)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func probeLatestRepoIssue(ctx context.Context, sourceID string, gh *github.Client, owner, repo string) (*github.Issue, error) {
	opts := &issueListByRepoOpts{
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
		PerPage:   issueCacheProbePageSize,
		Page:      1,
	}
	var issues []*github.Issue
	err := observeFetchCtx(ctx, sourceID, "issues.probe_latest", func(c context.Context) error {
		var fetchErr error
		issues, _, fetchErr = listRepoIssues(c, gh, owner, repo, opts)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	for _, issue := range issues {
		if issue != nil && !issue.IsPullRequest() {
			return issue, nil
		}
	}
	return nil, fmt.Errorf("no non-PR issues found in probe for %s/%s", owner, repo)
}
