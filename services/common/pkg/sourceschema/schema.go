// Package sourceschema defines versioned JSON documents collectors publish so UIs can build source forms.
package sourceschema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
)

const Kind = "SourceSchema"

// Arg describes one segment of a primary pattern (e.g. owner/name after repo:).
type Arg struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Format string `json:"format,omitempty"`
}

// PrimaryPattern is one way to build the primary part of source_spec (before +modifiers).
type PrimaryPattern struct {
	ID            string `json:"id"`
	SyntaxPrefix  string `json:"syntax_prefix"`
	Label         string `json:"label"`
	ValueTemplate string `json:"value_template"`
	Example       string `json:"example,omitempty"`
	Args          []Arg  `json:"args"`
}

// Modifier is an optional +token (e.g. +dependabot).
type Modifier struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`          // "boolean"
	SyntaxSuffix string `json:"syntax_suffix"` // e.g. "dependabot" for " +dependabot"
}

// CronHint documents optional cron_line behaviour for this collector.
type CronHint struct {
	Description string `json:"description,omitempty"`
	Optional    bool   `json:"optional"`
}

// Document is the wire JSON for AMQP and API body_json.
type Document struct {
	Cron            *CronHint        `json:"cron,omitempty"`
	Kind            string           `json:"kind"`
	SchemaVersion   string           `json:"schema_version"`
	CollectorType   string           `json:"collector_type"`
	PrimaryPatterns []PrimaryPattern `json:"primary_patterns"`
	Modifiers       []Modifier       `json:"modifiers"`
}

// Marshal returns canonical JSON bytes for publishing.
func Marshal(doc *Document) ([]byte, error) {
	return json.Marshal(doc)
}

// Publish sends doc to the IceHive topic exchange (default ex_icehive) on routing key collector.source_schema.<collector_type>.
func Publish(ctx context.Context, c *amqpctl.Client, doc *Document) error {
	if c == nil {
		return fmt.Errorf("sourceschema: nil AMQP client")
	}
	if doc == nil {
		return fmt.Errorf("sourceschema: nil document")
	}
	b, err := Marshal(doc)
	if err != nil {
		return err
	}
	rk := amqpctl.CollectorSourceSchemaRoutingKey(doc.CollectorType)
	return c.PublishJSON(ctx, rk, b)
}

// GitHubV1 is the collector-github source schema (matches collector-github parsing).
func GitHubV1() *Document {
	return &Document{
		Kind:          Kind,
		SchemaVersion: "1",
		CollectorType: "collector-github",
		PrimaryPatterns: []PrimaryPattern{
			{
				ID:           "repo",
				SyntaxPrefix: "repo:",
				Label:        "Single repository",
				Args: []Arg{
					{ID: "owner", Label: "Owner", Format: "github_login"},
					{ID: "name", Label: "Repository name", Format: "github_repo"},
				},
				ValueTemplate: "repo:{owner}/{name}",
				Example:       "repo:jamesread/faridoon",
			},
			{
				ID:           "org.repos",
				SyntaxPrefix: "org.repos:",
				Label:        "All repositories under user or organization",
				Args: []Arg{
					{ID: "login", Label: "User or organization", Format: "github_login"},
				},
				ValueTemplate: "org.repos:{login}",
				Example:       "org.repos:jamesread",
			},
		},
		Modifiers: []Modifier{
			{ID: "dependabot", Label: "Dependabot alerts", Kind: "boolean", SyntaxSuffix: "dependabot"},
			{ID: "pr", Label: "Pull requests (open + closed, non-archived repos)", Kind: "boolean", SyntaxSuffix: "pr"},
			{ID: "issue", Label: "Issues (open + closed, non-archived repos)", Kind: "boolean", SyntaxSuffix: "issue"},
		},
		Cron: &CronHint{
			Optional:    true,
			Description: "Leave empty for run-now-only sources (no scheduled polling).",
		},
	}
}

// TestdataV1 is a minimal schema for the testdata collector.
func TestdataV1() *Document {
	return &Document{
		Kind:            Kind,
		SchemaVersion:   "1",
		CollectorType:   "collector-testdata",
		PrimaryPatterns: []PrimaryPattern{},
		Modifiers:       []Modifier{},
		Cron: &CronHint{
			Optional:    false,
			Description: "Cron is not used by collector-testdata; emit interval is separate.",
		},
	}
}

// AzureV1 is a placeholder until Azure collection is implemented.
func AzureV1() *Document {
	return &Document{
		Kind:            Kind,
		SchemaVersion:   "1",
		CollectorType:   "collector-azure",
		PrimaryPatterns: []PrimaryPattern{},
		Modifiers:       []Modifier{},
		Cron:            &CronHint{Optional: true},
	}
}

// ImapV1 documents collector-imap (credentials from ICEHIVE_IMAP_* env; optional YAML poll interval).
func ImapV1() *Document {
	return &Document{
		Kind:            Kind,
		SchemaVersion:   "1",
		CollectorType:   "collector-imap",
		PrimaryPatterns: []PrimaryPattern{},
		Modifiers:       []Modifier{},
		Cron: &CronHint{
			Optional:    true,
			Description: "IMAP inbox is polled on poll_interval_seconds from YAML; controller cron is optional for future scheduling.",
		},
	}
}

// RssV1 documents collector-rss (RSS, Atom, and JSON Feed URLs via source_spec; optional ICEHIVE_RSS_USER_AGENT).
func RssV1() *Document {
	return &Document{
		Kind:          Kind,
		SchemaVersion: "1",
		CollectorType: "collector-rss",
		PrimaryPatterns: []PrimaryPattern{
			{
				ID:            "feed_url",
				SyntaxPrefix:  "feed:",
				Label:         "RSS, Atom, or JSON feed URL",
				Args:          []Arg{{ID: "url", Label: "Feed URL", Format: "https_url"}},
				ValueTemplate: "feed:{url}",
				Example: "feed:https://hnrss.org/frontpage (25 most recent articles when YAML items_max_per_feed is unset; " +
					"override count in YAML or use JSON {\"feed\":\"https://example.com/atom.xml\",\"articles_max\":50})",
			},
		},
		Modifiers: []Modifier{},
		Cron: &CronHint{
			Optional:    true,
			Description: "Leave empty for run-now-only sources (no scheduled polling).",
		},
	}
}

// JmapV1 documents collector-jmap (JMAP session/API URLs and bearer token from ICEHIVE_JMAP_* env; mailbox from source_spec or env).
func JmapV1() *Document {
	return &Document{
		Kind:          Kind,
		SchemaVersion: "1",
		CollectorType: "collector-jmap",
		PrimaryPatterns: []PrimaryPattern{
			{
				ID:            "mailbox",
				SyntaxPrefix:  "mailbox:",
				Label:         "Mailbox to list threads from",
				Args:          []Arg{{ID: "id", Label: "JMAP mailbox id", Format: "text"}},
				ValueTemplate: "mailbox:{id}",
				Example:       "mailbox:env, mailbox:inbox, or a concrete id",
			},
			{
				ID:            "inbox",
				SyntaxPrefix:  "mailbox:",
				Label:         "Inbox (RFC 8621 mailbox role)",
				Args:          []Arg{},
				ValueTemplate: "mailbox:inbox",
				Example:       "mailbox:inbox",
			},
		},
		Modifiers: []Modifier{},
		Cron: &CronHint{
			Optional:    true,
			Description: "Leave empty for run-now-only sources (no scheduled polling).",
		},
	}
}
