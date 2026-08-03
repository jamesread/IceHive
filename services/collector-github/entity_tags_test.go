package main

import (
	"encoding/json"
	"testing"

	"github.com/google/go-github/v71/github"
)

//gocyclo:ignore
func TestBuildGitRepoEntityDescriptionAndTags(t *testing.T) {
	t.Parallel()
	desc := "A test repository"
	topics := []string{"go", "icehive"}
	repo := &github.Repository{
		NodeID:      github.Ptr("R_kg"),
		Name:        github.Ptr("icehive"),
		FullName:    github.Ptr("org/icehive"),
		Description: &desc,
		Topics:      topics,
	}
	entity := buildGitRepoEntity(repo, nil, nil)
	if entity.Values["description"] != desc {
		t.Fatalf("description: got %v", entity.Values["description"])
	}
	var tags []string
	if err := json.Unmarshal([]byte(entity.Values["tags"].(string)), &tags); err != nil {
		t.Fatalf("tags json: %v", err)
	}
	if len(tags) != 2 || tags[0] != "go" || tags[1] != "icehive" {
		t.Fatalf("tags: %v", tags)
	}
	if _, ok := entity.Structure["tags"]; !ok {
		t.Fatal("structure missing tags")
	}
}

func TestRepoTagsJSONEmpty(t *testing.T) {
	t.Parallel()
	if repoTagsJSON(nil) != "[]" {
		t.Fatal("expected []")
	}
}
