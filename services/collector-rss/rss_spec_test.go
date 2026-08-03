package main

import (
	"strings"
	"testing"
)

//gocyclo:ignore
func TestParseRSSSourceSpec(t *testing.T) {
	t.Parallel()
	got, err := parseRSSSourceSpec("feed:https://example.com/atom.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got.FeedURL != "https://example.com/atom.xml" || got.ExplicitArticlesMax {
		t.Fatalf("plain feed: %+v", got)
	}

	j := `{"feed":"https://ex.test/feed","articles_max":12}`
	got2, err := parseRSSSourceSpec(j)
	if err != nil {
		t.Fatal(err)
	}
	if got2.FeedURL != "https://ex.test/feed" || !got2.ExplicitArticlesMax || got2.ArticlesMax != 12 {
		t.Fatalf("json explicit: %+v", got2)
	}

	got3, err := parseRSSSourceSpec(`{"feed_url":"https://ex.test/f"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got3.FeedURL != "https://ex.test/f" || got3.ExplicitArticlesMax {
		t.Fatalf("json implicit: %+v", got3)
	}
}

//gocyclo:ignore
func TestEffectiveArticlesMax(t *testing.T) {
	t.Parallel()
	plain := rssSourceSpec{FeedURL: "https://x"}
	if n := effectiveArticlesMax(plain, 0); n != defaultArticlesMax {
		t.Fatalf("plain yaml 0: %d", n)
	}
	if n := effectiveArticlesMax(plain, 100); n != 100 {
		t.Fatalf("plain yaml 100: %d", n)
	}
	explicit := rssSourceSpec{ExplicitArticlesMax: true, ArticlesMax: 40}
	if n := effectiveArticlesMax(explicit, 100); n != 40 {
		t.Fatalf("explicit 40 cap 100: %d", n)
	}
	if n := effectiveArticlesMax(explicit, 10); n != 10 {
		t.Fatalf("explicit 40 cap 10: %d", n)
	}
	explicitZero := rssSourceSpec{ExplicitArticlesMax: true, ArticlesMax: 0}
	if n := effectiveArticlesMax(explicitZero, 500); n != defaultArticlesMax {
		t.Fatalf("explicit 0 -> default: %d", n)
	}
}

func TestParseRSSSourceSpecJSONErrors(t *testing.T) {
	t.Parallel()
	_, err := parseRSSSourceSpec("{")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = parseRSSSourceSpec(`{"articles_max":10}`)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing feed err, got %v", err)
	}
}
