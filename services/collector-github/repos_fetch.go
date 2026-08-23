package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/go-github/v71/github"
)

// fetchReposForSource resolves a single repo (repo:owner/name) or all repos under a login (org.repos:login).
func fetchReposForSource(ctx context.Context, gh *github.Client, owner, repo string, allUnderLogin bool, sourceID string) ([]*github.Repository, error) {
	if !allUnderLogin {
		var r *github.Repository
		err := observeFetchCtx(ctx, sourceID, "repos.get", func(c context.Context) error {
			var innerErr error
			r, _, innerErr = gh.Repositories.Get(c, owner, repo)
			return innerErr
		})
		if err != nil {
			return nil, err
		}
		return []*github.Repository{r}, nil
	}
	return listAllReposUnderOwner(ctx, gh, owner, sourceID)
}

//gocyclo:ignore
func listAllReposUnderOwner(ctx context.Context, gh *github.Client, owner, sourceID string) ([]*github.Repository, error) {
	orgOpts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}
	var repos []*github.Repository
	var resp *github.Response
	err := observeFetchCtx(ctx, sourceID, "repos.list_by_org", func(c context.Context) error {
		var innerErr error
		repos, resp, innerErr = gh.Repositories.ListByOrg(c, owner, orgOpts)
		return innerErr
	})
	if err != nil {
		if isGitHub404(err) {
			return paginateUserRepos(ctx, gh, owner, sourceID)
		}
		return nil, err
	}
	out := append([]*github.Repository(nil), repos...)
	for resp != nil && resp.NextPage != 0 {
		orgOpts.Page = resp.NextPage
		err := observeFetchCtx(ctx, sourceID, "repos.list_by_org", func(c context.Context) error {
			var innerErr error
			repos, resp, innerErr = gh.Repositories.ListByOrg(c, owner, orgOpts)
			return innerErr
		})
		if err != nil {
			return nil, err
		}
		out = append(out, repos...)
	}
	return out, nil
}

func paginateUserRepos(ctx context.Context, gh *github.Client, owner, sourceID string) ([]*github.Repository, error) {
	opts := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100, Page: 1},
	}
	var out []*github.Repository
	for {
		var repos []*github.Repository
		var resp *github.Response
		err := observeFetchCtx(ctx, sourceID, "repos.list_by_user", func(c context.Context) error {
			var innerErr error
			repos, resp, innerErr = gh.Repositories.ListByUser(c, owner, opts)
			return innerErr
		})
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
