package main

import (
	"fmt"
	"strings"
)

const (
	prefixRepoLower     = "repo:"
	prefixOrgReposLower = "org.repos:"
)

// githubSourceOpts holds optional behaviors selected with "+modifier" tokens after the primary spec.
type githubSourceOpts struct {
	Dependabot bool
	PRs        bool
}

// splitSourceSpec splits "primary +mod1 +mod2" (modifiers are separated by a space followed by +).
func splitSourceSpec(sourceSpec string) (primary string, modifiers []string, err error) {
	s := strings.TrimSpace(sourceSpec)
	if s == "" {
		return "", nil, fmt.Errorf("empty source_spec")
	}
	parts := strings.Split(s, " +")
	primary = strings.TrimSpace(parts[0])
	if primary == "" {
		return "", nil, fmt.Errorf("empty source_spec primary")
	}
	for i := 1; i < len(parts); i++ {
		m := strings.TrimSpace(parts[i])
		if m == "" {
			return "", nil, fmt.Errorf("empty + modifier in source_spec")
		}
		if strings.ContainsAny(m, " \t\n") {
			return "", nil, fmt.Errorf("modifier %q must be a single token", m)
		}
		modifiers = append(modifiers, strings.ToLower(m))
	}
	return primary, modifiers, nil
}

func githubSourceOptsFromModifiers(modifiers []string) (githubSourceOpts, error) {
	seen := make(map[string]struct{}, len(modifiers))
	var out githubSourceOpts
	for _, m := range modifiers {
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		switch m {
		case "dependabot":
			out.Dependabot = true
		case "pr":
			out.PRs = true
		default:
			return githubSourceOpts{}, fmt.Errorf("unknown source_spec modifier %q (supported: dependabot, pr)", m)
		}
	}
	return out, nil
}

// parseGitHubSourceSpec parses the full source_spec: primary (repo:… or org.repos:…) plus optional "+modifier" tokens.
func parseGitHubSourceSpec(sourceSpec string) (owner, repo string, allReposUnderLogin bool, opts githubSourceOpts, err error) {
	primary, mods, err := splitSourceSpec(sourceSpec)
	if err != nil {
		return "", "", false, githubSourceOpts{}, err
	}
	o, err := githubSourceOptsFromModifiers(mods)
	if err != nil {
		return "", "", false, githubSourceOpts{}, err
	}
	owner, repo, allReposUnderLogin, err = parseRepoPrimary(primary)
	if err != nil {
		return "", "", false, githubSourceOpts{}, err
	}
	return owner, repo, allReposUnderLogin, o, nil
}

// parseRepoPrimary parses repo:owner/name (single repository) or org.repos:login (all repositories
// for that GitHub user or organization).
func parseRepoPrimary(primary string) (owner, repo string, allReposUnderLogin bool, err error) {
	s := strings.TrimSpace(primary)
	if s == "" {
		return "", "", false, fmt.Errorf("empty source_spec primary")
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, prefixOrgReposLower) {
		login := strings.TrimSpace(s[len(prefixOrgReposLower):])
		if login == "" {
			return "", "", false, fmt.Errorf(`missing login after %q`, "org.repos:")
		}
		if strings.ContainsAny(login, "/") {
			return "", "", false, fmt.Errorf("invalid org.repos login %q (expected a single user or org name)", login)
		}
		return login, "", true, nil
	}
	if strings.HasPrefix(lower, prefixRepoLower) {
		rest := strings.TrimSpace(s[len(prefixRepoLower):])
		if rest == "" {
			return "", "", false, fmt.Errorf(`missing owner/name after %q`, "repo:")
		}
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", "", false, fmt.Errorf(`invalid repo slug %q (expected owner/name for %q)`, rest, "repo:")
		}
		owner = strings.TrimSpace(parts[0])
		repo = strings.TrimSpace(parts[1])
		if repo == "*" {
			return "", "", false, fmt.Errorf(`use %q instead of %q to list all repositories under that login`, "org.repos:"+owner, "repo:"+owner+"/*")
		}
		if strings.Contains(repo, "*") {
			return "", "", false, fmt.Errorf(`wildcard repo names are not supported; use %q to list all repositories under that login`, "org.repos:"+owner)
		}
		return owner, repo, false, nil
	}
	return "", "", false, fmt.Errorf("unsupported source_spec primary %q (expected repo:owner/name or org.repos:login)", primary)
}
