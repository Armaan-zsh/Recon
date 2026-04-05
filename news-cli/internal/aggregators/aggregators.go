package aggregators

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"news-cli/internal/models"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

var DragnetQueries = []string{
	"\"zero-day\" OR \"0day\" OR ransomware",
	"malware AND (exploit OR compromised)",
	"(\"APT\" AND cyber) OR \"supply chain attack\"",
}

func FetchDragnetFeeds(ctx context.Context) []models.Article {
	var articles []models.Article
	client := &http.Client{Timeout: 10 * time.Second}
	fp := gofeed.NewParser()
	fp.Client = client
	fp.UserAgent = "Recon-Dragnet/1.0 (+https://github.com/recon-cli)"

	for _, query := range DragnetQueries {
		encodedQuery := url.QueryEscape(query)
		feedURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", encodedQuery)

		feed, err := fp.ParseURLWithContext(feedURL, ctx)
		if err != nil {
			continue
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

			title := item.Title
			if idx := strings.LastIndex(title, " - "); idx > 0 {
				title = title[:idx]
			}

			articles = append(articles, models.Article{
				Title:       title,
				Link:        item.Link,
				Description: desc,
				Content:     item.Content,
				Published:   pubDate,
				SourceName:  "[DRAGNET]",
				Score:       20,
			})
		}
	}

	return articles
}
