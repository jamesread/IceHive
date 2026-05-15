package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/sirupsen/logrus"
)

const (
	maxPullRequestsInEntity = 100
	maxPullRequestsFetch    = 500
)

// pullRequestsSnapshot holds PRs listed for a repo or an API error string if listing failed.
type pullRequestsSnapshot struct {
	PRs         []*github.PullRequest
	Err         string
	FetchCapped bool
}

func fetchPullRequestsForRepo(ctx context.Context, log *logrus.Logger, gh *github.Client, owner, repo string) pullRequestsSnapshot {
	var all []*github.PullRequest
	fetchCapped := false
	pagesFetched := 0
	for page := 1; ; page++ {
		if len(all) >= maxPullRequestsFetch {
			fetchCapped = true
			if log != nil {
				log.WithFields(logrus.Fields{
					"owner": owner, "repo": repo, "running_total": len(all),
					"max_pull_requests_fetch": maxPullRequestsFetch,
				}).Debug("github PR list: stopped pagination (running total cap)")
			}
			break
		}
		opts := &github.PullRequestListOptions{
			State:       "all",
			ListOptions: github.ListOptions{Page: page, PerPage: 100},
		}
		pulls, resp, err := gh.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			if log != nil {
				fields := logrus.Fields{
					"owner": owner, "repo": repo, "page": page,
				}
				var er *github.ErrorResponse
				if errors.As(err, &er) {
					if er.Response != nil {
						fields["http_status"] = er.Response.StatusCode
					}
					if m := strings.TrimSpace(er.Message); m != "" {
						fields["github_message"] = m
					}
				}
				log.WithError(err).WithFields(fields).Warn("GitHub PullRequests.List failed")
			}
			return pullRequestsSnapshot{Err: err.Error()}
		}
		pagesFetched++
		all = append(all, pulls...)
		if log != nil {
			next := 0
			if resp != nil {
				next = resp.NextPage
			}
			log.WithFields(logrus.Fields{
				"owner": owner, "repo": repo, "page": page,
				"batch_size": len(pulls), "running_total": len(all), "next_page": next,
			}).Debug("GitHub PullRequests.List page")
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	if log != nil {
		open := countOpenPullRequests(all)
		log.WithFields(logrus.Fields{
			"owner": owner, "repo": repo,
			"pages_fetched": pagesFetched, "pr_total": len(all), "pr_open": open,
			"fetch_capped": fetchCapped,
		}).Debug("GitHub pull requests fetch complete")
	}
	return pullRequestsSnapshot{PRs: all, FetchCapped: fetchCapped}
}

func countOpenPullRequests(prs []*github.PullRequest) int64 {
	var n int64
	for _, p := range prs {
		if strings.EqualFold(p.GetState(), "open") {
			n++
		}
	}
	return n
}

func simplifyPullRequests(prs []*github.PullRequest, max int) ([]map[string]any, bool) {
	if len(prs) == 0 {
		return nil, false
	}
	truncated := len(prs) > max
	if truncated {
		prs = prs[:max]
	}
	out := make([]map[string]any, 0, len(prs))
	for _, p := range prs {
		m := map[string]any{
			"number":   p.GetNumber(),
			"state":    p.GetState(),
			"title":    p.GetTitle(),
			"draft":    p.GetDraft(),
			"html_url": p.GetHTMLURL(),
			"locked":   p.GetLocked(),
		}
		if u := p.GetUser(); u != nil {
			m["user_login"] = u.GetLogin()
		}
		if h := p.GetHead(); h != nil {
			m["head_ref"] = h.GetRef()
			m["head_sha"] = h.GetSHA()
		}
		if b := p.GetBase(); b != nil {
			m["base_ref"] = b.GetRef()
			m["base_sha"] = b.GetSHA()
		}
		if t := p.GetCreatedAt(); !t.IsZero() {
			m["created_at"] = t.Format(time.RFC3339)
		}
		if t := p.GetUpdatedAt(); !t.IsZero() {
			m["updated_at"] = t.Format(time.RFC3339)
		}
		if t := p.GetMergedAt(); !t.IsZero() {
			m["merged_at"] = t.Format(time.RFC3339)
		}
		if t := p.GetClosedAt(); !t.IsZero() {
			m["closed_at"] = t.Format(time.RFC3339)
		}
		out = append(out, m)
	}
	return out, truncated
}
