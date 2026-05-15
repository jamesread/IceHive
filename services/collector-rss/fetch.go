package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

func newFeedParser(timeout time.Duration, userAgent string) *gofeed.Parser {
	fp := gofeed.NewParser()
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	fp.Client = &http.Client{Timeout: timeout}
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		ua = strings.TrimSpace(os.Getenv("ICEHIVE_RSS_USER_AGENT"))
	}
	if ua != "" {
		fp.UserAgent = ua
	}
	return fp
}

func fetchFeed(ctx context.Context, fp *gofeed.Parser, feedURL string) (*gofeed.Feed, error) {
	if fp == nil {
		fp = newFeedParser(90*time.Second, "")
	}
	return fp.ParseURLWithContext(feedURL, ctx)
}
