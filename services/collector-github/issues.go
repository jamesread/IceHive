package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/google/go-querystring/query"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
)

const maxIssuesFetch = 500

// issuesSnapshot holds GitHub issues listed for a repo or an API error string if listing failed.
type issuesSnapshot struct {
	Err         string
	Issues      []*github.Issue
	FetchCapped bool
}

// issueListByRepoOpts mirrors GitHub issue list query params, including cursor pagination (after).
type issueListByRepoOpts struct {
	Since     time.Time `url:"since,omitempty"`
	State     string    `url:"state,omitempty"`
	Sort      string    `url:"sort,omitempty"`
	Direction string    `url:"direction,omitempty"`
	After     string    `url:"after,omitempty"`
	Page      int       `url:"page,omitempty"`
	PerPage   int       `url:"per_page,omitempty"`
}

func listRepoIssues(ctx context.Context, gh *github.Client, owner, repo string, opts *issueListByRepoOpts) ([]*github.Issue, *github.Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues", owner, repo)
	v, err := query.Values(opts)
	if err != nil {
		return nil, nil, err
	}
	if enc := v.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := gh.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.squirrel-girl-preview")
	var issues []*github.Issue
	resp, err := gh.Do(ctx, req, &issues)
	return issues, resp, err
}

//gocyclo:ignore
func fetchIssuesForRepoSince(ctx context.Context, log *logrus.Logger, gh *github.Client, sourceID, owner, repo string, since time.Time) issuesSnapshot {
	var all []*github.Issue
	fetchCapped := false
	pagesFetched := 0
	after := ""
	page := 1
	for {
		if len(all) >= maxIssuesFetch {
			fetchCapped = true
			if log != nil {
				log.WithFields(logrus.Fields{
					"owner": owner, "repo": repo, "running_total": len(all),
					"max_issues_fetch": maxIssuesFetch,
				}).Debug("github issue list: stopped pagination (running total cap)")
			}
			break
		}
		opts := &issueListByRepoOpts{State: "all", PerPage: 100}
		if !since.IsZero() {
			opts.Since = since.UTC()
		}
		if after != "" {
			opts.After = after
		} else {
			opts.Page = page
		}
		var issues []*github.Issue
		var resp *github.Response
		fetchErr := observeFetchCtx(ctx, sourceID, "issues.list_by_repo", func(c context.Context) error {
			var err error
			issues, resp, err = listRepoIssues(c, gh, owner, repo, opts)
			return err
		})
		if fetchErr != nil {
			err := fetchErr
			if log != nil {
				fields := logrus.Fields{
					"owner": owner, "repo": repo, "page": page, "after": after,
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
				log.WithError(err).WithFields(fields).Warn("GitHub Issues.ListByRepo failed")
			}
			return issuesSnapshot{Err: err.Error()}
		}
		pagesFetched++
		for _, issue := range issues {
			if issue == nil || issue.IsPullRequest() {
				continue
			}
			all = append(all, issue)
			if len(all) >= maxIssuesFetch {
				fetchCapped = true
				break
			}
		}
		if fetchCapped {
			break
		}
		if log != nil {
			next := 0
			if resp != nil {
				next = resp.NextPage
			}
			log.WithFields(logrus.Fields{
				"owner": owner, "repo": repo, "page": page, "after": after != "",
				"batch_size": len(issues), "running_total": len(all), "next_page": next,
			}).Debug("GitHub Issues.ListByRepo page")
		}
		if resp == nil {
			break
		}
		if resp.After != "" {
			after = resp.After
			continue
		}
		if resp.NextPage != 0 {
			after = ""
			page = resp.NextPage
			continue
		}
		break
	}
	if log != nil {
		open := countOpenIssues(all)
		log.WithFields(logrus.Fields{
			"owner": owner, "repo": repo,
			"pages_fetched": pagesFetched, "issue_total": len(all), "issue_open": open,
			"fetch_capped": fetchCapped, "since": !since.IsZero(),
		}).Debug("GitHub issues fetch complete")
	}
	return issuesSnapshot{Issues: all, FetchCapped: fetchCapped}
}

func countOpenIssues(issues []*github.Issue) int64 {
	var n int64
	for _, issue := range issues {
		if strings.EqualFold(issue.GetState(), "open") {
			n++
		}
	}
	return n
}

//gocyclo:ignore
func issueLabelsJSON(labels []*github.Label) string {
	if len(labels) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if l == nil {
			continue
		}
		name := normLabelName(l.GetName())
		if name != "" {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return "[]"
	}
	return jsonScalar(out)
}

//gocyclo:ignore
func issueAssigneesJSON(users []*github.User) string {
	if len(users) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		login := norm.NFC.String(strings.TrimSpace(u.GetLogin()))
		if login != "" {
			out = append(out, login)
		}
	}
	if len(out) == 0 {
		return "[]"
	}
	return jsonScalar(out)
}

func normLabelName(s string) string {
	return norm.NFC.String(strings.TrimSpace(s))
}
