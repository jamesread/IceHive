package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"golang.org/x/text/unicode/norm"
)

const (
	entityTypeGitRepo          = "GitRepo"
	entityTypeDependabotIssue  = "DependabotIssue"
	maxSummaryLen              = 4096
	maxURLLen                  = 2048
)

func jsonScalar(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func sourceHashForUniqueID(uniqueID string) sourceHash {
	collector := norm.NFC.String(collectorGitHubType)
	uid := norm.NFC.String(uniqueID)
	sum := sha256.Sum256([]byte(uid + ":" + collector))
	return sourceHash{
		HashValue: hex.EncodeToString(sum[:]),
		HashType:  "sha256",
	}
}

func truncStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// gitHubRepoRecollectSpec returns a repo:-scoped source_spec hint for this repository, or nil if unknown.
func gitHubRepoRecollectSpec(repo *github.Repository) *string {
	if repo == nil {
		return nil
	}
	fn := strings.TrimSpace(repo.GetFullName())
	if fn == "" {
		return nil
	}
	s := norm.NFC.String(prefixRepoLower + fn)
	return &s
}

// dependabotIssueSourceUniqueID returns a stable id for this alert within GitHub.
// GitHub does not expose a Dependabot node_id on the REST alert; we use repo node id + alert number,
// or a digest of the alert HTML URL when number is missing.
func dependabotIssueSourceUniqueID(repoNodeID string, a *github.DependabotAlert) string {
	repoNodeID = norm.NFC.String(strings.TrimSpace(repoNodeID))
	if repoNodeID == "" || a == nil {
		return ""
	}
	if n := a.GetNumber(); n != 0 {
		return norm.NFC.String(fmt.Sprintf("%s:dependabot:%d", repoNodeID, n))
	}
	u := norm.NFC.String(strings.TrimSpace(a.GetHTMLURL()))
	if u == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(repoNodeID + ":" + u))
	return norm.NFC.String(fmt.Sprintf("%s:dependabot:url:%x", repoNodeID, sum[:12]))
}

type sourceHash struct {
	HashValue string `json:"hash_value"`
	HashType  string `json:"hash_type"`
}

type collectorMetadata struct {
	EntityType          string     `json:"entity_type"`
	SourceSystem        string     `json:"source_system"`
	SourceCollectorType string     `json:"source_collector_type"`
	SourceUniqueID      string     `json:"source_unique_id"`
	SourceHash          sourceHash `json:"source_hash"`
	ObservedUnixMS      int64      `json:"observed_unix_ms"`
	RecollectSpec       *string    `json:"recollect_spec"`
}

type fieldDescriptor struct {
	Type   string `json:"type"`
	Length int    `json:"length,omitempty"`
}

type entityMessage struct {
	Type          string                     `json:"type"`
	SchemaVersion string                     `json:"schema_version"`
	Metadata      collectorMetadata          `json:"collectormetadata"`
	Structure     map[string]fieldDescriptor `json:"structure"`
	Values        map[string]any             `json:"values"`
}

func buildGitRepoEntity(repo *github.Repository, dep *dependabotSnapshot, pr *pullRequestsSnapshot) entityMessage {
	uniqueID := norm.NFC.String(repo.GetNodeID())
	now := time.Now().UnixMilli()

	structure := map[string]fieldDescriptor{
		"name":            {Type: "string", Length: 255},
		"full_name":       {Type: "string", Length: 255},
		"stars":           {Type: "int64"},
		"forks":           {Type: "int64"},
		"is_private":      {Type: "bool"},
		"default_branch":  {Type: "string", Length: 255},
		"url":             {Type: "string", Length: 2048},
		"description":     {Type: "string", Length: 4096},
		"open_issues":     {Type: "int64"},
		"issue_count":     {Type: "int64"},
		"has_issues":      {Type: "bool"},
		"has_wiki":        {Type: "bool"},
		"archived":        {Type: "bool"},
		"disabled":        {Type: "bool"},
		"language":        {Type: "string", Length: 64},
		"license_spdx_id": {Type: "string", Length: 64},
	}
	values := repoToScalarValues(repo)

	if dep != nil {
		structure["dependabot_alert_count"] = fieldDescriptor{Type: "int64"}
		structure["dependabot_open_alert_count"] = fieldDescriptor{Type: "int64"}
		structure["dependabot_fetch_error"] = fieldDescriptor{Type: "string", Length: 4096}
		if dep.Err != "" {
			errStr := dep.Err
			if len(errStr) > 4096 {
				errStr = errStr[:4096]
			}
			values["dependabot_alert_count"] = int64(0)
			values["dependabot_open_alert_count"] = int64(0)
			values["dependabot_fetch_error"] = errStr
		} else {
			alerts := dep.Alerts
			values["dependabot_alert_count"] = int64(len(alerts))
			values["dependabot_open_alert_count"] = countOpenDependabotAlerts(alerts)
			values["dependabot_fetch_error"] = ""
		}
	}

	if pr != nil {
		structure["pull_request_count"] = fieldDescriptor{Type: "int64"}
		structure["pull_request_open_count"] = fieldDescriptor{Type: "int64"}
		structure["pull_requests"] = fieldDescriptor{Type: "string", Length: 1 << 20}
		structure["pull_requests_truncated"] = fieldDescriptor{Type: "bool"}
		structure["pull_requests_fetch_capped"] = fieldDescriptor{Type: "bool"}
		structure["pull_requests_fetch_error"] = fieldDescriptor{Type: "string", Length: 4096}
		if pr.Err != "" {
			errStr := pr.Err
			if len(errStr) > 4096 {
				errStr = errStr[:4096]
			}
			values["pull_request_count"] = int64(0)
			values["pull_request_open_count"] = int64(0)
			values["pull_requests"] = "[]"
			values["pull_requests_truncated"] = false
			values["pull_requests_fetch_capped"] = false
			values["pull_requests_fetch_error"] = errStr
		} else {
			list := pr.PRs
			open := countOpenPullRequests(list)
			values["pull_request_count"] = open
			values["pull_request_open_count"] = open
			simplified, listTrunc := simplifyPullRequests(list, maxPullRequestsInEntity)
			if len(simplified) == 0 {
				values["pull_requests"] = "[]"
			} else {
				values["pull_requests"] = jsonScalar(simplified)
			}
			values["pull_requests_truncated"] = listTrunc || pr.FetchCapped
			values["pull_requests_fetch_capped"] = pr.FetchCapped
			values["pull_requests_fetch_error"] = ""
		}
	}

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeGitRepo,
			SourceSystem:        "github",
			SourceCollectorType: collectorGitHubType,
			SourceUniqueID:      uniqueID,
			SourceHash:          sourceHashForUniqueID(uniqueID),
			ObservedUnixMS:      now,
			RecollectSpec:       gitHubRepoRecollectSpec(repo),
		},
		Structure: structure,
		Values:    values,
	}
}

func repoToScalarValues(repo *github.Repository) map[string]any {
	out := map[string]any{
		"name":           repo.GetName(),
		"full_name":      repo.GetFullName(),
		"stars":          int64(repo.GetStargazersCount()),
		"forks":          int64(repo.GetForksCount()),
		"is_private":     repo.GetPrivate(),
		"default_branch": repo.GetDefaultBranch(),
		"url":            repo.GetHTMLURL(),
		"description":    repo.GetDescription(),
		"open_issues":    int64(repo.GetOpenIssuesCount()),
		"issue_count":    int64(repo.GetOpenIssuesCount()),
		"has_issues":     repo.GetHasIssues(),
		"has_wiki":       repo.GetHasWiki(),
		"archived":       repo.GetArchived(),
		"disabled":       repo.GetDisabled(),
		"language":       repo.GetLanguage(),
	}
	if lic := repo.GetLicense(); lic != nil && lic.GetSPDXID() != "" {
		out["license_spdx_id"] = lic.GetSPDXID()
	} else {
		out["license_spdx_id"] = ""
	}
	return out
}

// buildDependabotIssueEntity maps one GitHub Dependabot alert to a DependabotIssue entity.
// values.git_repo_source_unique_id matches collectormetadata.source_unique_id on the GitRepo entity
// for the same repository (GitHub repository node_id, NFC).
func buildDependabotIssueEntity(repo *github.Repository, a *github.DependabotAlert) (entityMessage, bool) {
	if repo == nil || a == nil {
		return entityMessage{}, false
	}
	repoUID := norm.NFC.String(repo.GetNodeID())
	issueUID := dependabotIssueSourceUniqueID(repoUID, a)
	if issueUID == "" {
		return entityMessage{}, false
	}
	now := time.Now().UnixMilli()

	structure := map[string]fieldDescriptor{
		"git_repo_source_unique_id": {Type: "string", Length: 255},
		"git_repo_full_name":        {Type: "string", Length: 255},
		"git_repo_html_url":         {Type: "string", Length: maxURLLen},
		"alert_number":              {Type: "int64"},
		"html_url":                  {Type: "string", Length: maxURLLen},
		"api_url":                   {Type: "string", Length: maxURLLen},
		"state":                     {Type: "string", Length: 64},
		"severity":                  {Type: "string", Length: 64},
		"ghsa_id":                   {Type: "string", Length: 64},
		"cve_id":                    {Type: "string", Length: 64},
		"summary":                   {Type: "string", Length: maxSummaryLen},
		"package_ecosystem":         {Type: "string", Length: 128},
		"package_name":              {Type: "string", Length: 512},
		"manifest_path":             {Type: "string", Length: 2048},
		"scope":                     {Type: "string", Length: 64},
		// alert_* timestamps: avoid names created_at/updated_at — persister-mysql reserves those for row metadata.
		"alert_created_at":   {Type: "string", Length: 64},
		"alert_updated_at":   {Type: "string", Length: 64},
		"alert_dismissed_at": {Type: "string", Length: 64},
	}

	values := map[string]any{
		"git_repo_source_unique_id": repoUID,
		"git_repo_full_name":        norm.NFC.String(repo.GetFullName()),
		"git_repo_html_url":         truncStr(norm.NFC.String(repo.GetHTMLURL()), maxURLLen),
		"alert_number":              int64(a.GetNumber()),
		"html_url":                  truncStr(norm.NFC.String(a.GetHTMLURL()), maxURLLen),
		"api_url":                   truncStr(norm.NFC.String(a.GetURL()), maxURLLen),
		"state":                norm.NFC.String(a.GetState()),
		"alert_created_at":   "",
		"alert_updated_at":   "",
		"alert_dismissed_at": "",
	}
	if t := a.GetCreatedAt(); !t.IsZero() {
		values["alert_created_at"] = t.Format(time.RFC3339)
	}
	if t := a.GetUpdatedAt(); !t.IsZero() {
		values["alert_updated_at"] = t.Format(time.RFC3339)
	}
	if t := a.GetDismissedAt(); !t.IsZero() {
		values["alert_dismissed_at"] = t.Format(time.RFC3339)
	}

	severity := ""
	ghsa := ""
	cve := ""
	summary := ""
	if adv := a.GetSecurityAdvisory(); adv != nil {
		severity = norm.NFC.String(adv.GetSeverity())
		ghsa = norm.NFC.String(adv.GetGHSAID())
		cve = norm.NFC.String(adv.GetCVEID())
		summary = truncStr(norm.NFC.String(adv.GetSummary()), maxSummaryLen)
	}
	values["severity"] = severity
	values["ghsa_id"] = ghsa
	values["cve_id"] = cve
	values["summary"] = summary

	pkgEco := ""
	pkgName := ""
	manifestPath := ""
	scopeVal := ""
	if dep := a.GetDependency(); dep != nil {
		manifestPath = truncStr(norm.NFC.String(dep.GetManifestPath()), 2048)
		scopeVal = norm.NFC.String(dep.GetScope())
		if pkg := dep.GetPackage(); pkg != nil {
			pkgEco = norm.NFC.String(pkg.GetEcosystem())
			pkgName = truncStr(norm.NFC.String(pkg.GetName()), 512)
		}
	}
	values["package_ecosystem"] = pkgEco
	values["package_name"] = pkgName
	values["manifest_path"] = manifestPath
	values["scope"] = scopeVal

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeDependabotIssue,
			SourceSystem:        "github",
			SourceCollectorType: collectorGitHubType,
			SourceUniqueID:      issueUID,
			SourceHash:          sourceHashForUniqueID(issueUID),
			ObservedUnixMS:      now,
			RecollectSpec:       gitHubRepoRecollectSpec(repo),
		},
		Structure: structure,
		Values:    values,
	}, true
}
