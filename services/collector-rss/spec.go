package main

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const prefixFeedLower = "feed:"

// parseFeedSourceSpec returns the feed URL from a collection source source_spec.
// Accepts feed:https://host/path (recommended) or a bare http(s) URL.
func parseFeedSourceSpec(sourceSpec string) (feedURL string, err error) {
	s := strings.TrimSpace(sourceSpec)
	if s == "" {
		return "", fmt.Errorf("empty feed source_spec")
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, prefixFeedLower) {
		s = strings.TrimSpace(s[len(prefixFeedLower):])
	}
	if s == "" {
		return "", fmt.Errorf("missing URL after %q", prefixFeedLower)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("parse feed URL: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("feed URL must use http or https scheme, got %q", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("feed URL missing host in %q", sourceSpec)
	}
	return norm.NFC.String(u.String()), nil
}
