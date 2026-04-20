package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"news-cli/internal/aggregators"
	"news-cli/internal/clusterer"
	"news-cli/internal/database"
	"news-cli/internal/extractor"
	"news-cli/internal/models"
	"news-cli/internal/scorer"
	"news-cli/internal/textutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/errgroup"
)

var dateRegex = regexp.MustCompile(`/(20\d{2})/(0[1-9]|1[0-2])/`)

func FetchAll(ctx context.Context, keywords []string, torProxy string, db *database.IntelligenceDB, feedData []byte) (models.FetchResult, error) {
	start := time.Now()
	feeds, err := LoadFeeds(feedData)
	if err != nil {
		return models.FetchResult{}, err
	}

	ext := extractor.NewExtractor()
	var (
		articles []models.Article
		mu       sync.Mutex
		fetched  int
	)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(500)

	for _, src := range feeds {
		src := src
		g.Go(func() error {
			feedArticles, err := fetchSingleFeed(ctx, src, torProxy, db)
			if err != nil {
				return nil
			}

			mu.Lock()
			fetched++
			mu.Unlock()

			validArticles := []models.Article{}
			for _, a := range feedArticles {
				scorer.ScoreArticle(&a, keywords)
				if a.Score > 5 {
					validArticles = append(validArticles, a)

					if db != nil {
						ents := ext.ExtractEntities(a)
						_ = db.SaveArticle(a, ents)
					}
				}
			}

			mu.Lock()
			articles = append(articles, validArticles...)
			mu.Unlock()
			return nil
		})
	}

	g.Go(func() error {
		dragnetArticles := aggregators.FetchDragnetFeeds(ctx)
		var validArticles []models.Article
		for _, a := range dragnetArticles {
			scorer.ScoreArticle(&a, keywords)
			validArticles = append(validArticles, a)

			if db != nil {
				ents := ext.ExtractEntities(a)
				_ = db.SaveArticle(a, ents)
			}
		}
		mu.Lock()
		articles = append(articles, validArticles...)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		configDir, _ := os.UserConfigDir()
		robinPath := filepath.Join(configDir, "recon", "robin_intel.json")
		if _, err := os.Stat(robinPath); err == nil {
			robinArticles, err := IngestRobinIntel(robinPath)
			if err == nil {
				mu.Lock()
				for _, a := range robinArticles {
					articles = append(articles, a)
				}
				mu.Unlock()
			}
		}
		return nil
	})

	_ = g.Wait()

	c := clusterer.NewClusterer(3)
	clusters := c.ClusterArticles(articles)

	finalArticles := []models.Article{}
	for _, cl := range clusters {
		finalArticles = append(finalArticles, cl.PrimaryArticle)
	}

	sort.Slice(finalArticles, func(i, j int) bool {
		if finalArticles[i].Score == finalArticles[j].Score {
			return finalArticles[i].Published.After(finalArticles[j].Published)
		}
		return finalArticles[i].Score > finalArticles[j].Score
	})

	return models.FetchResult{
		Articles:     finalArticles,
		TotalFeeds:   len(feeds),
		FetchedFeeds: fetched,
		Duration:     time.Since(start),
	}, nil
}

func LoadFeeds(data []byte) ([]models.FeedSource, error) {
	var payload struct {
		Links []models.FeedSource `json:"links"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload.Links, nil
}

func fetchSingleFeed(ctx context.Context, source models.FeedSource, torProxy string, db *database.IntelligenceDB) ([]models.Article, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	fp := gofeed.NewParser()
	fp.UserAgent = "Recon/2.0 (+https://github.com/recon-cli)"

	if strings.HasSuffix(strings.Split(source.URL, "/")[2], ".onion") && torProxy != "" {
		proxyURL, err := url.Parse(torProxy)
		if err == nil {
			dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
			if err == nil {
				client.Transport = &http.Transport{Dial: dialer.Dial}
				client.Timeout = 20 * time.Second
			}
		}
	}
	fp.Client = client

	var etag, lastModified string
	if db != nil {
		etag, lastModified, _ = db.GetFeedCache(source.URL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fp.UserAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if db != nil {
			_ = db.SetFeedCache(source.URL, etag, lastModified)
		}
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("feed http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	newETag := resp.Header.Get("ETag")
	newLastModified := resp.Header.Get("Last-Modified")
	if newETag == "" {
		newETag = etag
	}
	if newLastModified == "" {
		newLastModified = lastModified
	}
	if db != nil {
		_ = db.SetFeedCache(source.URL, newETag, newLastModified)
	}

	feed, err := fp.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	articles := []models.Article{}
	for _, item := range feed.Items {
		var pubDate time.Time
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			pubDate = *item.UpdatedParsed
		} else {
			// HEURISTIC: Extract from URL (Blogger/WP standard)
			matches := dateRegex.FindStringSubmatch(item.Link)
			if len(matches) == 3 {
				year, _ := strconv.Atoi(matches[1])
				month, _ := strconv.Atoi(matches[2])
				pubDate = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			} else {
				// No date found? Set to 2000-01-01 to ignore it
				pubDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			}
		}

		// Reject future-dated articles
		if pubDate.After(time.Now().Add(1 * time.Hour)) {
			continue
		}

		desc := textutil.Truncate(textutil.PlainText(item.Description), 500)
		title := textutil.PlainText(item.Title)
		if title == "" {
			title = item.Title
		}

		articles = append(articles, models.Article{
			Title:       title,
			Link:        item.Link,
			Description: desc,
			Content:     item.Content,
			Published:   pubDate,
			SourceName:  source.Name,
		})
	}

	return articles, nil
}
