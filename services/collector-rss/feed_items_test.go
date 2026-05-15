package main

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestSortFeedItemsNewestFirst(t *testing.T) {
	t.Parallel()
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	items := []*gofeed.Item{
		{Title: "old", PublishedParsed: &old},
		{Title: "new", PublishedParsed: &newer},
		{Title: "nodate"},
	}
	sortFeedItemsNewestFirst(items)
	if items[0].Title != "new" || items[1].Title != "old" || items[2].Title != "nodate" {
		t.Fatalf("order: %#v", []string{items[0].Title, items[1].Title, items[2].Title})
	}
}
