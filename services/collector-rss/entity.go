package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/text/unicode/norm"
)

const entityTypeRssFeed = "RssFeed"
const entityTypeRssArticle = "RssArticle"

type sourceHash struct {
	HashValue string `json:"hash_value"`
	HashType  string `json:"hash_type"`
}

type collectorMetadata struct {
	RecollectSpec       *string    `json:"recollect_spec"`
	EntityType          string     `json:"entity_type"`
	SourceSystem        string     `json:"source_system"`
	SourceCollectorType string     `json:"source_collector_type"`
	SourceUniqueID      string     `json:"source_unique_id"`
	SourceHash          sourceHash `json:"source_hash"`
	ObservedUnixMS      int64      `json:"observed_unix_ms"`
}

type fieldDescriptor struct {
	Type   string `json:"type"`
	Length int    `json:"length,omitempty"`
}

type entityMessage struct {
	Type          string                     `json:"type"`
	SchemaVersion string                     `json:"schema_version"`
	Structure     map[string]fieldDescriptor `json:"structure"`
	Values        map[string]any             `json:"values"`
	Metadata      collectorMetadata          `json:"collectormetadata"`
}

func sourceHashFor(uniqueID string) sourceHash {
	collector := norm.NFC.String(collectorRssType)
	uid := norm.NFC.String(uniqueID)
	sum := sha256.Sum256([]byte(uid + ":" + collector))
	return sourceHash{HashValue: hex.EncodeToString(sum[:]), HashType: "sha256"}
}

func truncStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func entryLink(item *gofeed.Item) string {
	if item == nil {
		return ""
	}
	if u := strings.TrimSpace(item.Link); u != "" {
		return norm.NFC.String(u)
	}
	for _, u := range item.Links {
		if t := strings.TrimSpace(u); t != "" {
			return norm.NFC.String(t)
		}
	}
	return ""
}

func entryAuthors(item *gofeed.Item) string {
	if item == nil {
		return ""
	}
	var parts []string
	for _, a := range item.Authors {
		parts = appendAuthorName(parts, a)
	}
	parts = appendAuthorName(parts, item.Author)
	if len(parts) == 0 {
		return ""
	}
	return truncStr(strings.Join(parts, ", "), 1024)
}

func appendAuthorName(parts []string, author *gofeed.Person) []string {
	if author == nil {
		return parts
	}
	if name := strings.TrimSpace(author.Name); name != "" {
		return append(parts, norm.NFC.String(name))
	}
	return parts
}

func entryPublishedUnixMs(item *gofeed.Item) int64 {
	if item == nil {
		return 0
	}
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC().UnixMilli()
	}
	return 0
}

func entryUpdatedUnixMs(item *gofeed.Item) int64 {
	if item == nil {
		return 0
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC().UnixMilli()
	}
	return 0
}

func entryPubKeyString(item *gofeed.Item) string {
	if item == nil {
		return ""
	}
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC().Format(time.RFC3339Nano)
	}
	return norm.NFC.String(strings.TrimSpace(item.Published))
}

func entrySummary(item *gofeed.Item, max int) string {
	if item == nil {
		return ""
	}
	body := strings.TrimSpace(item.Content)
	if body == "" {
		body = strings.TrimSpace(item.Description)
	}
	return truncStr(norm.NFC.String(body), max)
}

// feedEntrySourceUniqueID is stable per collection source, feed URL, and item identity (guid, link, published).
// feedRecollectSpec is a single-feed source_spec hint (collector-rss feed:… syntax).
func feedRecollectSpec(feedURL string) *string {
	u := strings.TrimSpace(feedURL)
	if u == "" {
		return nil
	}
	s := norm.NFC.String("feed:" + u)
	return &s
}

func feedEntrySourceUniqueID(collectionSourceID, feedURL string, item *gofeed.Item) string {
	if item == nil {
		return ""
	}
	guid := norm.NFC.String(strings.TrimSpace(item.GUID))
	link := entryLink(item)
	pub := entryPubKeyString(item)
	sum := sha256.Sum256([]byte(
		norm.NFC.String(collectionSourceID) + "\x00" +
			norm.NFC.String(feedURL) + "\x00" +
			guid + "\x00" +
			link + "\x00" +
			pub,
	))
	return norm.NFC.String(hex.EncodeToString(sum[:]))
}

func rssFeedSourceUniqueID(collectionSourceID, feedURL string) string {
	sum := sha256.Sum256([]byte(
		norm.NFC.String(collectionSourceID) + "\x00" +
			norm.NFC.String(feedURL) + "\x00" +
			"rss_feed_v1",
	))
	return norm.NFC.String(hex.EncodeToString(sum[:]))
}

func feedUpdatedUnixMs(feed *gofeed.Feed) int64 {
	if feed == nil {
		return 0
	}
	if feed.UpdatedParsed != nil {
		return feed.UpdatedParsed.UTC().UnixMilli()
	}
	return 0
}

func feedSiteLink(feed *gofeed.Feed) string {
	if feed == nil {
		return ""
	}
	if u := strings.TrimSpace(feed.Link); u != "" {
		return norm.NFC.String(u)
	}
	if u := strings.TrimSpace(feed.FeedLink); u != "" {
		return norm.NFC.String(u)
	}
	return firstNonEmptyNormalizedLink(feed.Links)
}

func firstNonEmptyNormalizedLink(links []string) string {
	for _, u := range links {
		if t := strings.TrimSpace(u); t != "" {
			return norm.NFC.String(t)
		}
	}
	return ""
}

// buildRssFeedEntity maps the fetched feed document to one RssFeed entity per collection run.
func buildRssFeedEntity(collectionSourceID, feedURL string, feed *gofeed.Feed, feedType string, articlesSelected int) (entityMessage, bool) {
	if feed == nil {
		return entityMessage{}, false
	}
	uniqueID := rssFeedSourceUniqueID(collectionSourceID, feedURL)
	if uniqueID == "" {
		return entityMessage{}, false
	}
	now := time.Now().UnixMilli()
	feedTitle := strings.TrimSpace(feed.Title)
	ft := strings.TrimSpace(feedType)
	if ft == "" {
		ft = "unknown"
	}

	structure := map[string]fieldDescriptor{
		"collection_source_id":    {Type: "string", Length: 255},
		"feed_url":                {Type: "string", Length: 2048},
		"feed_title":              {Type: "string", Length: 2048},
		"feed_format":             {Type: "string", Length: 32},
		"feed_site_link":          {Type: "string", Length: 2048},
		"feed_description":        {Type: "string", Length: 8192},
		"feed_language":           {Type: "string", Length: 64},
		"articles_selected_count": {Type: "int64"},
		"feed_updated_unix_ms":    {Type: "int64"},
	}

	values := map[string]any{
		"collection_source_id":    norm.NFC.String(collectionSourceID),
		"feed_url":                feedURL,
		"feed_title":              truncStr(norm.NFC.String(feedTitle), 2048),
		"feed_format":             truncStr(norm.NFC.String(ft), 32),
		"feed_site_link":          truncStr(feedSiteLink(feed), 2048),
		"feed_description":        truncStr(norm.NFC.String(strings.TrimSpace(feed.Description)), 8192),
		"feed_language":           truncStr(norm.NFC.String(strings.TrimSpace(feed.Language)), 64),
		"articles_selected_count": int64(articlesSelected),
		"feed_updated_unix_ms":    feedUpdatedUnixMs(feed),
	}

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeRssFeed,
			SourceSystem:        "http_feed",
			SourceCollectorType: collectorRssType,
			SourceUniqueID:      uniqueID,
			SourceHash:          sourceHashFor(uniqueID),
			ObservedUnixMS:      now,
			RecollectSpec:       feedRecollectSpec(feedURL),
		},
		Structure: structure,
		Values:    values,
	}, true
}

// buildRssArticleEntity maps one syndicated entry to a RssArticle entity (RSS, Atom, or JSON Feed via gofeed).
func buildRssArticleEntity(collectionSourceID, feedURL, feedTitle, feedType string, item *gofeed.Item) (entityMessage, bool) {
	if item == nil {
		return entityMessage{}, false
	}
	uniqueID := feedEntrySourceUniqueID(collectionSourceID, feedURL, item)
	if uniqueID == "" {
		return entityMessage{}, false
	}
	now := time.Now().UnixMilli()

	structure := map[string]fieldDescriptor{
		"collection_source_id": {Type: "string", Length: 255},
		"feed_url":             {Type: "string", Length: 2048},
		"feed_title":           {Type: "string", Length: 2048},
		"feed_format":          {Type: "string", Length: 32},
		"entry_guid":           {Type: "string", Length: 2048},
		"entry_link":           {Type: "string", Length: 2048},
		"title":                {Type: "string", Length: 2048},
		"entry_summary":        {Type: "string", Length: 16384},
		"author":               {Type: "string", Length: 1024},
		"categories":           {Type: "string", Length: 4096},
		"published_unix_ms":    {Type: "int64"},
		"updated_unix_ms":      {Type: "int64"},
	}

	catJoined := ""
	if len(item.Categories) > 0 {
		catJoined = truncStr(norm.NFC.String(strings.Join(item.Categories, ", ")), 4096)
	}

	values := map[string]any{
		"collection_source_id": norm.NFC.String(collectionSourceID),
		"feed_url":             feedURL,
		"feed_title":           truncStr(norm.NFC.String(feedTitle), 2048),
		"feed_format":          truncStr(norm.NFC.String(feedType), 32),
		"entry_guid":           truncStr(norm.NFC.String(strings.TrimSpace(item.GUID)), 2048),
		"entry_link":           truncStr(entryLink(item), 2048),
		"title":                truncStr(norm.NFC.String(strings.TrimSpace(item.Title)), 2048),
		"entry_summary":        entrySummary(item, 16384),
		"author":               entryAuthors(item),
		"categories":           catJoined,
		"published_unix_ms":    entryPublishedUnixMs(item),
		"updated_unix_ms":      entryUpdatedUnixMs(item),
	}

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeRssArticle,
			SourceSystem:        "http_feed",
			SourceCollectorType: collectorRssType,
			SourceUniqueID:      uniqueID,
			SourceHash:          sourceHashFor(uniqueID),
			ObservedUnixMS:      now,
			RecollectSpec:       feedRecollectSpec(feedURL),
		},
		Structure: structure,
		Values:    values,
	}, true
}
