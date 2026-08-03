package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v71/github"
)

func repoOwnerLogin(repo *github.Repository) string {
	if o := repo.GetOwner(); o != nil {
		if login := o.GetLogin(); login != "" {
			return login
		}
	}
	fn := repo.GetFullName()
	if i := strings.IndexByte(fn, '/'); i > 0 {
		return fn[:i]
	}
	return ""
}

// dependabotSnapshot holds Dependabot alerts for a repo or an API error string if listing failed.
type dependabotSnapshot struct {
	Err    string
	Alerts []*github.DependabotAlert
}

//gocyclo:ignore
func fetchDependabotAlertsForRepo(ctx context.Context, gh *github.Client, owner, repo string) dependabotSnapshot {
	// Dependabot alerts use cursor pagination (Link rel="next" with ?cursor=...), not ?page=.
	// state=open: server-side filter so dismissed/fixed alerts are not fetched or emitted.
	opts := &github.ListAlertsOptions{
		State:             github.Ptr("open"),
		ListCursorOptions: github.ListCursorOptions{PerPage: 100},
	}
	var all []*github.DependabotAlert
	for {
		alerts, resp, err := gh.Dependabot.ListRepoAlerts(ctx, owner, repo, opts)
		if err != nil {
			return dependabotSnapshot{Err: err.Error()}
		}
		all = append(all, alerts...)
		if resp == nil || resp.Cursor == "" {
			break
		}
		opts.Cursor = resp.Cursor
	}
	return dependabotSnapshot{Alerts: all}
}

func countOpenDependabotAlerts(alerts []*github.DependabotAlert) int64 {
	var n int64
	for _, a := range alerts {
		if strings.EqualFold(a.GetState(), "open") {
			n++
		}
	}
	return n
}
