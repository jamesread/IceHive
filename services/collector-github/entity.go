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
	entityTypeGitHubIssue      = "GitHubIssue"
	maxSummaryLen              = 4096
	maxBodyLen                 = 16384
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

// githubIssueSourceUniqueID returns a stable id for this issue within GitHub.
func githubIssueSourceUniqueID(repoNodeID string, issue *github.Issue) string {
	if issue == nil {
		return ""
	}
	if nid := norm.NFC.String(strings.TrimSpace(issue.GetNodeID())); nid != "" {
		return nid
	}
	repoNodeID = norm.NFC.String(strings.TrimSpace(repoNodeID))
	if repoNodeID == "" {
		return ""
	}
	if n := issue.GetNumber(); n != 0 {
		return norm.NFC.String(fmt.Sprintf("%s:issue:%d", repoNodeID, n))
	}
	u := norm.NFC.String(strings.TrimSpace(issue.GetHTMLURL()))
	if u == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(repoNodeID + ":" + u))
	return norm.NFC.String(fmt.Sprintf("%s:issue:url:%x", repoNodeID, sum[:12]))
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

	structure := gitRepoScalarStructure()
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

// repoTagsJSON returns a JSON array string of GitHub repository topics (UI "tags").
func repoTagsJSON(topics []string) string {
	if len(topics) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(topics))
	for _, t := range topics {
		t = norm.NFC.String(strings.TrimSpace(t))
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return "[]"
	}
	return jsonScalar(out)
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

// buildGitHubIssueEntity maps one GitHub issue to a GitHubIssue entity.
// values.git_repo_source_unique_id matches collectormetadata.source_unique_id on the GitRepo entity
// for the same repository (GitHub repository node_id, NFC).
func buildGitHubIssueEntity(repo *github.Repository, issue *github.Issue) (entityMessage, bool) {
	if repo == nil || issue == nil || issue.IsPullRequest() {
		return entityMessage{}, false
	}
	repoUID := norm.NFC.String(repo.GetNodeID())
	issueUID := githubIssueSourceUniqueID(repoUID, issue)
	if issueUID == "" {
		return entityMessage{}, false
	}
	now := time.Now().UnixMilli()

	structure := map[string]fieldDescriptor{
		"git_repo_source_unique_id": {Type: "string", Length: 255},
		"git_repo_full_name":        {Type: "string", Length: 255},
		"git_repo_html_url":         {Type: "string", Length: maxURLLen},
		"issue_number":              {Type: "int64"},
		"html_url":                  {Type: "string", Length: maxURLLen},
		"api_url":                   {Type: "string", Length: maxURLLen},
		"state":                     {Type: "string", Length: 64},
		"state_reason":              {Type: "string", Length: 64},
		"title":                     {Type: "string", Length: 2048},
		"body":                      {Type: "string", Length: maxBodyLen},
		"user_login":                {Type: "string", Length: 255},
		"locked":                    {Type: "bool"},
		"draft":                     {Type: "bool"},
		"comments_count":            {Type: "int64"},
		"labels":                    {Type: "string", Length: 4096},
		"assignees":                 {Type: "string", Length: 4096},
		"milestone_title":           {Type: "string", Length: 512},
		"milestone_number":          {Type: "int64"},
		"issue_created_at":          {Type: "string", Length: 64},
		"issue_updated_at":          {Type: "string", Length: 64},
		"issue_closed_at":           {Type: "string", Length: 64},
	}

	userLogin := ""
	if u := issue.GetUser(); u != nil {
		userLogin = norm.NFC.String(u.GetLogin())
	}
	milestoneTitle := ""
	var milestoneNumber int64
	if m := issue.GetMilestone(); m != nil {
		milestoneTitle = truncStr(norm.NFC.String(m.GetTitle()), 512)
		milestoneNumber = int64(m.GetNumber())
	}

	values := map[string]any{
		"git_repo_source_unique_id": repoUID,
		"git_repo_full_name":        norm.NFC.String(repo.GetFullName()),
		"git_repo_html_url":         truncStr(norm.NFC.String(repo.GetHTMLURL()), maxURLLen),
		"issue_number":              int64(issue.GetNumber()),
		"html_url":                  truncStr(norm.NFC.String(issue.GetHTMLURL()), maxURLLen),
		"api_url":                   truncStr(norm.NFC.String(issue.GetURL()), maxURLLen),
		"state":                     norm.NFC.String(issue.GetState()),
		"state_reason":              norm.NFC.String(issue.GetStateReason()),
		"title":                     truncStr(norm.NFC.String(issue.GetTitle()), 2048),
		"body":                      truncStr(norm.NFC.String(issue.GetBody()), maxBodyLen),
		"user_login":                userLogin,
		"locked":                    issue.GetLocked(),
		"draft":                     issue.GetDraft(),
		"comments_count":            int64(issue.GetComments()),
		"labels":                    issueLabelsJSON(issue.Labels),
		"assignees":                 issueAssigneesJSON(issue.Assignees),
		"milestone_title":           milestoneTitle,
		"milestone_number":          milestoneNumber,
		"issue_created_at":          "",
		"issue_updated_at":          "",
		"issue_closed_at":           "",
	}
	if t := issue.GetCreatedAt(); !t.IsZero() {
		values["issue_created_at"] = t.Format(time.RFC3339)
	}
	if t := issue.GetUpdatedAt(); !t.IsZero() {
		values["issue_updated_at"] = t.Format(time.RFC3339)
	}
	if t := issue.GetClosedAt(); !t.IsZero() {
		values["issue_closed_at"] = t.Format(time.RFC3339)
	}

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeGitHubIssue,
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
