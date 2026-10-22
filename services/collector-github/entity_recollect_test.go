package main

import (
	"testing"

	"github.com/google/go-github/v71/github"
)

func requireGitHubRepoRecollectSpec(t *testing.T, p *string, want string) {
	t.Helper()
	if p == nil {
		t.Fatal("expected non-nil spec")
	}
	if *p != want {
		t.Fatalf("got %q", *p)
	}
}

func TestGitHubRepoRecollectSpec(t *testing.T) {
	t.Parallel()
	if gitHubRepoRecollectSpec(nil) != nil {
		t.Fatal("expected nil for nil repo")
	}
	if gitHubRepoRecollectSpec(&github.Repository{}) != nil {
		t.Fatal("expected nil when full_name empty")
	}
	fn := "jamesread/faridoon"
	requireGitHubRepoRecollectSpec(t, gitHubRepoRecollectSpec(&github.Repository{FullName: &fn}), "repo:jamesread/faridoon")
}
