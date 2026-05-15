package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/go-github/v71/github"
)

// fetchReposForSource resolves a single repo (repo:owner/name) or all repos under a login (org.repos:login).
func fetchReposForSource(ctx context.Context, gh *github.Client, owner, repo string, allUnderLogin bool) ([]*github.Repository, error) {
	if !allUnderLogin {
		r, _, err := gh.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return nil, err
		}
		return []*github.Repository{r}, nil
	}
	return listAllReposUnderOwner(ctx, gh, owner)
}

func listAllReposUnderOwner(ctx context.Context, gh *github.Client, owner string) ([]*github.Repository, error) {
	orgOpts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}
	repos, resp, err := gh.Repositories.ListByOrg(ctx, owner, orgOpts)
	if err != nil {
		if isGitHub404(err) {
			return paginateUserRepos(ctx, gh, owner)
		}
		return nil, err
	}
	out := append([]*github.Repository(nil), repos...)
	for resp != nil && resp.NextPage != 0 {
		orgOpts.Page = resp.NextPage
		repos, resp, err = gh.Repositories.ListByOrg(ctx, owner, orgOpts)
		if err != nil {
			return nil, err
		}
		out = append(out, repos...)
	}
	return out, nil
}

func paginateUserRepos(ctx context.Context, gh *github.Client, owner string) ([]*github.Repository, error) {
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}
	var out []*github.Repository
	for {
		repos, resp, err := gh.Repositories.List(ctx, owner, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, repos...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func isGitHub404(err error) bool {
	var e *github.ErrorResponse
	return errors.As(err, &e) && e.Response != nil && e.Response.StatusCode == http.StatusNotFound
}
