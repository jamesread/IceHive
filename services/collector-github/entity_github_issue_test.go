package main

import (
	"encoding/json"
	"testing"

	"github.com/google/go-github/v71/github"
)

func TestBuildGitHubIssueEntity(t *testing.T) {
	t.Parallel()
	repoNode := "R_kgDOGH123"
	issueNode := "I_kwDOGH456"
	repoFull := "icehive/icehive"
	repoURL := "https://github.com/icehive/icehive"
	num := 42
	title := "Fix collector"
	body := "Issue body text"
	state := "open"
	login := "alice"
	labelName := "bug"
	milestoneTitle := "v1.0"
	milestoneNum := 3

	repo := &github.Repository{
		NodeID:   &repoNode,
		FullName: &repoFull,
		HTMLURL:  &repoURL,
	}
	issue := &github.Issue{
		NodeID:  &issueNode,
		Number:  &num,
		Title:   &title,
		Body:    &body,
		State:   &state,
		User:    &github.User{Login: github.String(login)},
		Labels:  []*github.Label{{Name: github.String(labelName)}},
		Assignees: []*github.User{
			{Login: github.String("bob")},
		},
		Milestone: &github.Milestone{
			Title:  github.String(milestoneTitle),
			Number: github.Int(milestoneNum),
		},
	}

	entity, ok := buildGitHubIssueEntity(repo, issue)
	if !ok {
		t.Fatal("expected ok")
	}
	if entity.Metadata.EntityType != entityTypeGitHubIssue {
		t.Fatalf("entity_type: got %q", entity.Metadata.EntityType)
	}
	if entity.Metadata.SourceUniqueID != issueNode {
		t.Fatalf("source_unique_id: got %q", entity.Metadata.SourceUniqueID)
	}
	if entity.Values["git_repo_source_unique_id"] != repoNode {
		t.Fatalf("git_repo_source_unique_id: got %v", entity.Values["git_repo_source_unique_id"])
	}
	if entity.Values["issue_number"] != int64(42) {
		t.Fatalf("issue_number: got %v", entity.Values["issue_number"])
	}
	if entity.Values["user_login"] != login {
		t.Fatalf("user_login: got %v", entity.Values["user_login"])
	}
	if entity.Values["milestone_title"] != milestoneTitle {
		t.Fatalf("milestone_title: got %v", entity.Values["milestone_title"])
	}

	var labels []string
	if err := json.Unmarshal([]byte(entity.Values["labels"].(string)), &labels); err != nil {
		t.Fatalf("labels json: %v", err)
	}
	if len(labels) != 1 || labels[0] != labelName {
		t.Fatalf("labels: got %v", labels)
	}
}

func TestBuildGitHubIssueEntitySkipsPullRequest(t *testing.T) {
	t.Parallel()
	repo := &github.Repository{NodeID: github.String("R_x"), FullName: github.String("o/r")}
	prURL := "https://github.com/o/r/pull/1"
	issue := &github.Issue{
		NodeID: github.String("I_x"),
		Number: github.Int(1),
		PullRequestLinks: &github.PullRequestLinks{
			HTMLURL: &prURL,
		},
	}
	if _, ok := buildGitHubIssueEntity(repo, issue); ok {
		t.Fatal("expected pull request to be skipped")
	}
}

func TestGitHubIssueSourceUniqueIDFallback(t *testing.T) {
	t.Parallel()
	repoNode := "R_kgDOGH123"
	num := 7
	issue := &github.Issue{Number: &num}
	got := githubIssueSourceUniqueID(repoNode, issue)
	want := "R_kgDOGH123:issue:7"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
