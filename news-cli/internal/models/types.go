package models

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"time"
)

type Article struct {
	Title       string    `json:"Title"`
	Link        string    `json:"Link"`
	Description string    `json:"Description"`
	Content     string    `json:"Content"`
	Published   time.Time `json:"Published"`
	SourceName  string    `json:"SourceName"`
	Score       int       `json:"Score"`
	IoCs        []string  `json:"IoCs,omitempty"`
	PatchLink   string    `json:"PatchLink,omitempty"`
}

func (a Article) Hash() string {
	u, err := url.Parse(a.Link)
	base := a.Link
	if err == nil {
		u.RawQuery = ""
		u.Fragment = ""
		base = u.String()
	}
	h := sha256.New()
	h.Write([]byte(base))
	return hex.EncodeToString(h.Sum(nil))
}

type FeedSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type FetchResult struct {
	Articles     []Article
	TotalFeeds   int
	FetchedFeeds int
	FailedFeeds  int
	Duration     time.Duration
}

type ClusterGroup struct {
	ID              string
	PrimaryArticle  Article
	RelatedArticles []Article
}

type Entity struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Mentions int    `json:"mentions"`
}

type EntityNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Mentions int    `json:"mentions"`
}

type EntityEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Weight int    `json:"weight"`
}

type TimelineEntry struct {
	Date   time.Time
	Title  string
	Source string
	Link   string
}
