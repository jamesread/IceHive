package main

import (
	"testing"

	"github.com/google/go-github/v71/github"
)

func TestCountOpenIssues(t *testing.T) {
	t.Parallel()
	open := "open"
	closed := "closed"
	issues := []*github.Issue{
		{State: &open},
		{State: &closed},
		{State: &open},
	}
	if n := countOpenIssues(issues); n != 2 {
		t.Fatalf("got %d want 2", n)
	}
}

func TestIssueLabelsJSON(t *testing.T) {
	t.Parallel()
	if issueLabelsJSON(nil) != "[]" {
		t.Fatal("expected empty array")
	}
	got := issueLabelsJSON([]*github.Label{
		{Name: github.String("bug")},
		{Name: github.String("enhancement")},
	})
	if got != `["bug","enhancement"]` {
		t.Fatalf("got %q", got)
	}
}
