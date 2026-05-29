package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v71/github"
)

// enrichRepo loads full repository metadata when list responses omit description or topics.
func enrichRepo(ctx context.Context, gh *github.Client, repo *github.Repository) (*github.Repository, error) {
	if repo == nil {
		return nil, nil
	}
	if strings.TrimSpace(repo.GetDescription()) != "" && len(repo.Topics) > 0 {
		return repo, nil
	}
	owner := repoOwnerLogin(repo)
	name := repo.GetName()
	if owner == "" || name == "" {
		return repo, nil
	}
	full, _, err := gh.Repositories.Get(ctx, owner, name)
	if err != nil {
		return repo, err
	}
	if len(full.Topics) == 0 {
		if topics, _, topicsErr := gh.Repositories.ListAllTopics(ctx, owner, name); topicsErr == nil {
			full.Topics = topics
		}
	}
	return full, nil
}
