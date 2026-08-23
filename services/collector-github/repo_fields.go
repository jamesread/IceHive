package main

import (
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"golang.org/x/text/unicode/norm"
)

const maxCustomPropertiesJSON = 16384

// gitRepoScalarStructure is the field schema for GitRepo values from Repositories.Get.
func gitRepoScalarStructure() map[string]fieldDescriptor {
	return map[string]fieldDescriptor{
		"name":               {Type: "string", Length: 255},
		"full_name":          {Type: "string", Length: 255},
		"owner_login":        {Type: "string", Length: 255},
		"organization_login": {Type: "string", Length: 255},
		"stars":              {Type: "int64"},
		"forks":              {Type: "int64"},
		"subscribers_count":  {Type: "int64"},
		"network_count":      {Type: "int64"},
		"size":               {Type: "int64"},
		"is_private":         {Type: "bool"},
		"visibility":         {Type: "string", Length: 32},
		"fork":               {Type: "bool"},
		"parent_full_name":   {Type: "string", Length: 255},
		"source_full_name":   {Type: "string", Length: 255},
		"default_branch":     {Type: "string", Length: 255},
		"url":                {Type: "string", Length: maxURLLen},
		"homepage":           {Type: "string", Length: maxURLLen},
		"clone_url":          {Type: "string", Length: maxURLLen},
		"ssh_url":            {Type: "string", Length: maxURLLen},
		"description":        {Type: "string", Length: 4096},
		"tags":               {Type: "string", Length: 4096},
		// repo_* timestamps: avoid names created_at/updated_at — persister-mysql reserves those for row metadata.
		"repo_created_at":        {Type: "string", Length: 64},
		"repo_updated_at":        {Type: "string", Length: 64},
		"pushed_at":              {Type: "string", Length: 64},
		"open_issues":            {Type: "int64"},
		"issue_count":            {Type: "int64"},
		"has_issues":             {Type: "bool"},
		"has_wiki":               {Type: "bool"},
		"has_projects":           {Type: "bool"},
		"has_pages":              {Type: "bool"},
		"has_discussions":        {Type: "bool"},
		"is_template":            {Type: "bool"},
		"archived":               {Type: "bool"},
		"disabled":               {Type: "bool"},
		"allow_forking":          {Type: "bool"},
		"delete_branch_on_merge": {Type: "bool"},
		"allow_squash_merge":     {Type: "bool"},
		"allow_merge_commit":     {Type: "bool"},
		"allow_rebase_merge":     {Type: "bool"},
		"allow_auto_merge":       {Type: "bool"},
		"language":               {Type: "string", Length: 64},
		"license_spdx_id":        {Type: "string", Length: 64},
		"license_key":            {Type: "string", Length: 64},
		"license_name":           {Type: "string", Length: 255},
		"security_and_analysis":  {Type: "string", Length: 4096},
		"custom_properties":      {Type: "string", Length: maxCustomPropertiesJSON},
	}
}

func repoToScalarValues(repo *github.Repository) map[string]any {
	vis := norm.NFC.String(strings.TrimSpace(repo.GetVisibility()))
	if vis == "" {
		if repo.GetPrivate() {
			vis = "private"
		} else {
			vis = "public"
		}
	}
	out := map[string]any{
		"name":                   repo.GetName(),
		"full_name":              repo.GetFullName(),
		"owner_login":            repoOwnerLogin(repo),
		"organization_login":     repoOrganizationLogin(repo),
		"stars":                  int64(repo.GetStargazersCount()),
		"forks":                  int64(repo.GetForksCount()),
		"subscribers_count":      int64(repo.GetSubscribersCount()),
		"network_count":          int64(repo.GetNetworkCount()),
		"size":                   int64(repo.GetSize()),
		"is_private":             repo.GetPrivate(),
		"visibility":             vis,
		"fork":                   repo.GetFork(),
		"parent_full_name":       repoLinkedFullName(repo.GetParent()),
		"source_full_name":       repoLinkedFullName(repo.GetSource()),
		"default_branch":         repo.GetDefaultBranch(),
		"url":                    repo.GetHTMLURL(),
		"homepage":               truncStr(norm.NFC.String(repo.GetHomepage()), maxURLLen),
		"clone_url":              truncStr(norm.NFC.String(repo.GetCloneURL()), maxURLLen),
		"ssh_url":                truncStr(norm.NFC.String(repo.GetSSHURL()), maxURLLen),
		"description":            truncStr(norm.NFC.String(repo.GetDescription()), 4096),
		"tags":                   repoTagsJSON(repo.Topics),
		"repo_created_at":        githubTimeRFC3339(repo.GetCreatedAt()),
		"repo_updated_at":        githubTimeRFC3339(repo.GetUpdatedAt()),
		"pushed_at":              githubTimeRFC3339(repo.GetPushedAt()),
		"open_issues":            int64(repo.GetOpenIssuesCount()),
		"issue_count":            int64(repo.GetOpenIssuesCount()),
		"has_issues":             repo.GetHasIssues(),
		"has_wiki":               repo.GetHasWiki(),
		"has_projects":           repo.GetHasProjects(),
		"has_pages":              repo.GetHasPages(),
		"has_discussions":        repo.GetHasDiscussions(),
		"is_template":            repo.GetIsTemplate(),
		"archived":               repo.GetArchived(),
		"disabled":               repo.GetDisabled(),
		"allow_forking":          repo.GetAllowForking(),
		"delete_branch_on_merge": repo.GetDeleteBranchOnMerge(),
		"allow_squash_merge":     repo.GetAllowSquashMerge(),
		"allow_merge_commit":     repo.GetAllowMergeCommit(),
		"allow_rebase_merge":     repo.GetAllowRebaseMerge(),
		"allow_auto_merge":       repo.GetAllowAutoMerge(),
		"language":               repo.GetLanguage(),
		"license_spdx_id":        "",
		"license_key":            "",
		"license_name":           "",
		"security_and_analysis":  securityAndAnalysisJSON(repo.GetSecurityAndAnalysis()),
		"custom_properties":      customPropertiesJSON(repo.CustomProperties),
	}
	if lic := repo.GetLicense(); lic != nil {
		out["license_spdx_id"] = lic.GetSPDXID()
		out["license_key"] = lic.GetKey()
		out["license_name"] = lic.GetName()
	}
	return out
}

func githubTimeRFC3339(t github.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func repoLinkedFullName(r *github.Repository) string {
	if r == nil {
		return ""
	}
	return norm.NFC.String(strings.TrimSpace(r.GetFullName()))
}

func repoOrganizationLogin(repo *github.Repository) string {
	if repo == nil {
		return ""
	}
	if org := repo.GetOrganization(); org != nil {
		if login := strings.TrimSpace(org.GetLogin()); login != "" {
			return norm.NFC.String(login)
		}
	}
	return ""
}

//gocyclo:ignore
func securityAndAnalysisJSON(s *github.SecurityAndAnalysis) string {
	if s == nil {
		return "{}"
	}
	m := map[string]string{}
	putSecurityStatus := func(key string, status string) {
		status = norm.NFC.String(strings.TrimSpace(status))
		if status != "" {
			m[key] = status
		}
	}
	if x := s.GetAdvancedSecurity(); x != nil {
		putSecurityStatus("advanced_security", x.GetStatus())
	}
	if x := s.GetSecretScanning(); x != nil {
		putSecurityStatus("secret_scanning", x.GetStatus())
	}
	if x := s.GetSecretScanningPushProtection(); x != nil {
		putSecurityStatus("secret_scanning_push_protection", x.GetStatus())
	}
	if x := s.GetDependabotSecurityUpdates(); x != nil {
		putSecurityStatus("dependabot_security_updates", x.GetStatus())
	}
	if x := s.GetSecretScanningValidityChecks(); x != nil {
		putSecurityStatus("secret_scanning_validity_checks", x.GetStatus())
	}
	if len(m) == 0 {
		return "{}"
	}
	return jsonScalar(m)
}

func customPropertiesJSON(props map[string]interface{}) string {
	if len(props) == 0 {
		return "{}"
	}
	s := jsonScalar(props)
	if len(s) > maxCustomPropertiesJSON {
		return s[:maxCustomPropertiesJSON]
	}
	return s
}
