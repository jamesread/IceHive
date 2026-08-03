package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-github/v71/github"
)

func TestIssueListUnchanged(t *testing.T) {
	t.Parallel()
	issueID := "I_kw123"
	updated := "2026-06-18T08:16:14Z"
	cache := &repoIssueCacheFile{
		LatestIssueID:   issueID,
		LatestUpdatedAt: updated,
		Issues:          map[string]cachedIssueEntry{},
	}
	ts, _ := time.Parse(time.RFC3339, updated)
	probe := &github.Issue{
		NodeID:    github.Ptr(issueID),
		UpdatedAt: &github.Timestamp{Time: ts},
	}
	if !issueListUnchanged(cache, probe) {
		t.Fatal("expected unchanged")
	}
	probe.NodeID = github.Ptr("I_other")
	if issueListUnchanged(cache, probe) {
		t.Fatal("expected changed when issue id differs")
	}
}

func TestMergeIssueCachePublishesNewAndUpdated(t *testing.T) {
	t.Parallel()
	repoUID := "R_repo"
	oldUpdated := "2026-06-01T10:00:00Z"
	newUpdated := "2026-06-18T08:16:14Z"
	oldIssue := &github.Issue{
		NodeID:    github.Ptr("I_old"),
		Number:    github.Ptr(1),
		UpdatedAt: mustGitHubTime(oldUpdated),
	}
	oldRaw, _ := json.Marshal(oldIssue)
	cache := &repoIssueCacheFile{
		Issues: map[string]cachedIssueEntry{
			"I_old": {UpdatedAt: oldUpdated, Issue: oldRaw},
		},
	}
	updatedIssue := &github.Issue{
		NodeID:    github.Ptr("I_old"),
		Number:    github.Ptr(1),
		UpdatedAt: mustGitHubTime(newUpdated),
	}
	newIssue := &github.Issue{
		NodeID:    github.Ptr("I_new"),
		Number:    github.Ptr(2),
		UpdatedAt: mustGitHubTime(newUpdated),
	}
	toPublish, merged, skipped := mergeIssueCache(cache, []*github.Issue{updatedIssue, newIssue}, repoUID)
	if skipped != 0 {
		t.Fatalf("skipped: %d", skipped)
	}
	if len(toPublish) != 2 {
		t.Fatalf("published: %d", len(toPublish))
	}
	if merged.LatestIssueID != "I_new" && merged.LatestIssueID != "I_old" {
		t.Fatalf("latest issue id: %q", merged.LatestIssueID)
	}
}

func TestMergeIssueCacheSkipsUnchanged(t *testing.T) {
	t.Parallel()
	repoUID := "R_repo"
	updated := "2026-06-18T08:16:14Z"
	issue := &github.Issue{
		NodeID:    github.Ptr("I_same"),
		Number:    github.Ptr(3),
		UpdatedAt: mustGitHubTime(updated),
	}
	raw, _ := json.Marshal(issue)
	cache := &repoIssueCacheFile{
		Issues: map[string]cachedIssueEntry{
			"I_same": {UpdatedAt: updated, Issue: raw},
		},
	}
	toPublish, _, skipped := mergeIssueCache(cache, []*github.Issue{issue}, repoUID)
	if len(toPublish) != 0 || skipped != 1 {
		t.Fatalf("publish=%d skipped=%d", len(toPublish), skipped)
	}
}

func TestIssueCollectorSaveLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := &issueCollector{dir: dir, enabled: true}
	cache := &repoIssueCacheFile{
		Version:         issueCacheVersion,
		RepoFullName:    "OliveTin/OliveTin",
		LatestIssueID:   "I_x",
		LatestUpdatedAt: "2026-06-18T08:16:14Z",
		Issues: map[string]cachedIssueEntry{
			"I_x": {UpdatedAt: "2026-06-18T08:16:14Z", Issue: json.RawMessage(`{"node_id":"I_x"}`)},
		},
	}
	if err := c.save("OliveTin", "OliveTin", cache); err != nil {
		t.Fatal(err)
	}
	loaded, err := c.load("OliveTin", "OliveTin")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LatestIssueID != "I_x" || len(loaded.Issues) != 1 {
		t.Fatalf("loaded: %+v", loaded)
	}
}

func mustGitHubTime(s string) *github.Timestamp {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &github.Timestamp{Time: t}
}
