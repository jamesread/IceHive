package main

import (
	"strings"
	"testing"
)

func TestFeedRecollectSpec(t *testing.T) {
	t.Parallel()
	if feedRecollectSpec("") != nil {
		t.Fatal("empty url")
	}
	if feedRecollectSpec("   ") != nil {
		t.Fatal("blank url")
	}
	p := feedRecollectSpec("https://example.com/news.atom")
	if p == nil || *p != "feed:https://example.com/news.atom" {
		t.Fatalf("got %v", p)
	}
}

//gocyclo:ignore
func TestParseFeedSourceSpec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "feed:https://example.com/atom.xml", want: "https://example.com/atom.xml"},
		{in: "FEED:https://example.com/", want: "https://example.com/"},
		{in: "https://news.ycombinator.com/rss", want: "https://news.ycombinator.com/rss"},
		{in: "  feed:http://localhost:8080/feed  ", want: "http://localhost:8080/feed"},
		{in: "", wantErr: true},
		{in: "feed:", wantErr: true},
		{in: "ftp://example.com/x", wantErr: true},
		{in: "https://", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc.in, "/", "_"), func(t *testing.T) {
			t.Parallel()
			got, err := parseFeedSourceSpec(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFeedSourceSpec: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
