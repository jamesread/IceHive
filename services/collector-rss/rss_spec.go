package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// defaultArticlesMax is used when YAML items_max_per_feed is unset and source_spec does not pin a count.
const defaultArticlesMax = 25

// rssSourceSpec is the normalized collection intent for one RSS source.
type rssSourceSpec struct {
	FeedURL             string
	ArticlesMax         int
	ExplicitArticlesMax bool // true when JSON source_spec set "articles_max" explicitly
}

// parseRSSSourceSpec parses source_spec as either:
//   - JSON: {"feed":"https://…"} or {"feed_url":"https://…","articles_max":50} (articles_max optional).
//   - Legacy URL: feed:https://… or bare https://… (article count from YAML items_max_per_feed, else 25).
func parseRSSSourceSpec(sourceSpec string) (rssSourceSpec, error) {
	s := strings.TrimSpace(sourceSpec)
	if s == "" {
		return rssSourceSpec{}, fmt.Errorf("empty feed source_spec")
	}
	if strings.HasPrefix(s, "{") {
		var j struct {
			Feed        string `json:"feed"`
			FeedURL     string `json:"feed_url"`
			ArticlesMax *int   `json:"articles_max"`
		}
		if err := json.Unmarshal([]byte(s), &j); err != nil {
			return rssSourceSpec{}, fmt.Errorf("parse rss source_spec json: %w", err)
		}
		raw := strings.TrimSpace(j.Feed)
		if raw == "" {
			raw = strings.TrimSpace(j.FeedURL)
		}
		if raw == "" {
			return rssSourceSpec{}, fmt.Errorf("rss json source_spec: missing feed or feed_url")
		}
		feedURL, err := parseFeedSourceSpec(raw)
		if err != nil {
			return rssSourceSpec{}, err
		}
		if j.ArticlesMax != nil {
			return rssSourceSpec{
				FeedURL:             feedURL,
				ArticlesMax:         *j.ArticlesMax,
				ExplicitArticlesMax: true,
			}, nil
		}
		return rssSourceSpec{FeedURL: feedURL}, nil
	}
	feedURL, err := parseFeedSourceSpec(s)
	if err != nil {
		return rssSourceSpec{}, err
	}
	return rssSourceSpec{FeedURL: feedURL}, nil
}

// effectiveArticlesMax decides how many articles to emit after sorting by recency.
// - Plain feed: or JSON without articles_max — use yamlCap when set (>0), else defaultArticlesMax (25).
// - JSON with explicit articles_max — use that value, positive, capped by yamlCap when yamlCap > 0.
func effectiveArticlesMax(spec rssSourceSpec, yamlCap int) int {
	if spec.ExplicitArticlesMax {
		n := spec.ArticlesMax
		if n <= 0 {
			n = defaultArticlesMax
		}
		if yamlCap > 0 && n > yamlCap {
			return yamlCap
		}
		return n
	}
	if yamlCap > 0 {
		return yamlCap
	}
	return defaultArticlesMax
}
