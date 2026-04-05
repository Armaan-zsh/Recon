package models

import (
	"time"
)

// Article represents a single scored news item.
type Article struct {
	Title       string
	Link        string
	Description string
	Content     string // Raw content for extraction
	Published   time.Time
	SourceName  string
	Score       int
	Hash        string
}

// FeedSource represents a single RSS/Atom feed to fetch.
type FeedSource struct {
	Name string
	URL  string
}

// FetchResult holds the results from fetching all feeds.
type FetchResult struct {
	Articles   []Article
	TotalFeeds int
	FetchedFeeds int
	Duration   time.Duration
}

// ClusterGroup represents a group of similar articles.
type ClusterGroup struct {
	ID             string
	PrimaryArticle Article
	RelatedArticles []Article
}

// Entity represents an extracted security entity.
type Entity struct {
	Name string
	Type string // MALWARE, APT, CVE, TARGET, INFRA, NATION_STATE
}
