package main

import "testing"

func TestParseGitHubSourceSpecIssueModifier(t *testing.T) {
	t.Parallel()
	owner, repo, all, opts, err := parseGitHubSourceSpec("repo:icehive/icehive +issue")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "icehive" || repo != "icehive" || all {
		t.Fatalf("owner=%q repo=%q all=%v", owner, repo, all)
	}
	if !opts.Issues || opts.Dependabot || opts.PRs {
		t.Fatalf("opts: %+v", opts)
	}
}

func TestParseGitHubSourceSpecAllModifiers(t *testing.T) {
	t.Parallel()
	_, _, _, opts, err := parseGitHubSourceSpec("org.repos:icehive +dependabot +pr +issue")
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Dependabot || !opts.PRs || !opts.Issues {
		t.Fatalf("opts: %+v", opts)
	}
}

func TestParseGitHubSourceSpecUnknownModifier(t *testing.T) {
	t.Parallel()
	_, _, _, _, err := parseGitHubSourceSpec("repo:o/r +foo")
	if err == nil {
		t.Fatal("expected error")
	}
}
