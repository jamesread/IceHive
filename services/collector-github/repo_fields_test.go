package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-github/v71/github"
)

//gocyclo:ignore
func TestRepoToScalarValuesExtended(t *testing.T) {
	t.Parallel()
	vis := "internal"
	parentFN := "upstream/parent"
	sourceFN := "upstream/source"
	homepage := "https://example.com"
	created := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	adv := "enabled"
	repo := &github.Repository{
		NodeID:     new("R_kg"),
		Name:       new("forked"),
		FullName:   new("org/forked"),
		Fork:       new(true),
		Visibility: &vis,
		Homepage:   &homepage,
		Size:       new(42),
		CreatedAt:  &github.Timestamp{Time: created},
		Parent:     &github.Repository{FullName: &parentFN},
		Source:     &github.Repository{FullName: &sourceFN},
		SecurityAndAnalysis: &github.SecurityAndAnalysis{
			AdvancedSecurity: &github.AdvancedSecurity{Status: &adv},
		},
		CustomProperties: map[string]any{"tier": "prod"},
	}
	owner := "org"
	repo.Owner = &github.User{Login: &owner}

	vals := repoToScalarValues(repo)
	if vals["fork"] != true {
		t.Fatalf("fork: %v", vals["fork"])
	}
	if vals["visibility"] != "internal" {
		t.Fatalf("visibility: %v", vals["visibility"])
	}
	if vals["parent_full_name"] != parentFN {
		t.Fatalf("parent: %v", vals["parent_full_name"])
	}
	if vals["source_full_name"] != sourceFN {
		t.Fatalf("source: %v", vals["source_full_name"])
	}
	if vals["owner_login"] != "org" {
		t.Fatalf("owner_login: %v", vals["owner_login"])
	}
	if vals["repo_created_at"] != created.Format(time.RFC3339) {
		t.Fatalf("repo_created_at: %v", vals["repo_created_at"])
	}
	var sec map[string]string
	if err := json.Unmarshal([]byte(vals["security_and_analysis"].(string)), &sec); err != nil {
		t.Fatal(err)
	}
	if sec["advanced_security"] != "enabled" {
		t.Fatalf("security: %v", sec)
	}
	var props map[string]any
	if err := json.Unmarshal([]byte(vals["custom_properties"].(string)), &props); err != nil {
		t.Fatal(err)
	}
	if props["tier"] != "prod" {
		t.Fatalf("custom_properties: %v", props)
	}
}

func TestGitRepoScalarStructureCoversValues(t *testing.T) {
	t.Parallel()
	structure := gitRepoScalarStructure()
	repo := &github.Repository{
		NodeID:   new("R"),
		Name:     new("n"),
		FullName: new("o/n"),
	}
	for key := range repoToScalarValues(repo) {
		if _, ok := structure[key]; !ok {
			t.Fatalf("structure missing %q", key)
		}
	}
}
