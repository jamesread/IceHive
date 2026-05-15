package main

import (
	"sort"
	"strings"

	"github.com/mmcdole/gofeed"
)

// itemRecencyUnixMs prefers the latest of published or updated wall times (UTC ms), or 0 if unknown.
func itemRecencyUnixMs(item *gofeed.Item) int64 {
	if item == nil {
		return 0
	}
	p := entryPublishedUnixMs(item)
	u := entryUpdatedUnixMs(item)
	if u > p {
		return u
	}
	return p
}

// sortFeedItemsNewestFirst sorts items in place by recency (newest first). Unknown dates sort last.
func sortFeedItemsNewestFirst(items []*gofeed.Item) {
	sort.SliceStable(items, func(i, j int) bool {
		ai, aj := items[i], items[j]
		ti, tj := itemRecencyUnixMs(ai), itemRecencyUnixMs(aj)
		if ti != tj {
			return ti > tj
		}
		return strings.Compare(entryLink(ai), entryLink(aj)) < 0
	})
}
