package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v71/github"
)

// enrichRepo loads full repository metadata when list responses omit description or topics.
//
//gocyclo:ignore
func enrichRepo(ctx context.Context, gh *github.Client, repo *github.Repository, sourceID string) (*github.Repository, error) {
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
	var full *github.Repository
	err := observeFetchCtx(ctx, sourceID, "repos.get_enrich", func(c context.Context) error {
		var innerErr error
		full, _, innerErr = gh.Repositories.Get(c, owner, name)
		return innerErr
	})
	if err != nil {
		return repo, err
	}
	if len(full.Topics) == 0 {
		_ = observeFetchCtx(ctx, sourceID, "repos.list_topics", func(c context.Context) error {
			topics, _, topicsErr := gh.Repositories.ListAllTopics(c, owner, name)
			if topicsErr == nil {
				full.Topics = topics
			}
			return topicsErr
		})
	}
	return full, nil
}
