package main

import (
	"testing"

	"github.com/google/go-github/v71/github"
)

func TestGitHubRepoRecollectSpec(t *testing.T) {
	t.Parallel()
	if gitHubRepoRecollectSpec(nil) != nil {
		t.Fatal("expected nil for nil repo")
	}
	if gitHubRepoRecollectSpec(&github.Repository{}) != nil {
		t.Fatal("expected nil when full_name empty")
	}
	fn := "jamesread/faridoon"
	p := gitHubRepoRecollectSpec(&github.Repository{FullName: &fn})
	if p == nil {
		t.Fatal("expected non-nil spec")
	}
	if *p != "repo:jamesread/faridoon" {
		t.Fatalf("got %q", *p)
	}
}
