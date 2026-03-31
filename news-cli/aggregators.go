package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

var dragnetQueries = []string{
	"zero-day OR 0day OR ransomware",
	"malware AND (exploit OR compromised)",
	"APT OR \"supply chain attack\"",
}

// FetchDragnetFeeds generates targeted Google News RSS feeds and fetches them.
func FetchDragnetFeeds(ctx context.Context, cfg *AppConfig) []Article {
	var articles []Article
	client := &http.Client{Timeout: 10 * time.Second}
	fp := gofeed.NewParser()
	fp.Client = client
	fp.UserAgent = "Recon-Dragnet/1.0 (+https://github.com/recon-cli)"

	for _, query := range dragnetQueries {
		encodedQuery := url.QueryEscape(query)
		feedURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", encodedQuery)
		
		feed, err := fp.ParseURLWithContext(feedURL, ctx)
		if err != nil {
			continue // Skip failed queries silently
		}

		for _, item := range feed.Items {
			pubDate := time.Now()
			if item.PublishedParsed != nil {
				pubDate = *item.PublishedParsed
			}

			desc := item.Description
			if len(desc) > 500 {
				desc = desc[:500] + "..."
			}

			// Clean up Google News title (removes "- Site Name")
			title := item.Title
			if idx := strings.LastIndex(title, " - "); idx > 0 {
				title = title[:idx]
			}

			articles = append(articles, Article{
				Title:       title,
				Link:        item.Link,
				Description: desc,
				Content:     item.Content,
				Published:   pubDate,
				SourceName:  "[DRAGNET]", // Unique identifier for UI
				Score:       20,          // Base Dragnet bonus
			})
		}
	}

	return articles
}
