package fetcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"news-cli/internal/aggregators"
	"news-cli/internal/clusterer"
	"news-cli/internal/database"
	"news-cli/internal/extractor"
	"news-cli/internal/models"
	"news-cli/internal/scorer"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/errgroup"
)

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

	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	for _, src := range feeds {
		src := src
		g.Go(func() error {
			feedArticles, err := fetchSingleFeed(ctx, src, torProxy)
			if err != nil {
				return nil
			}

			mu.Lock()
			fetched++
			mu.Unlock()

			validArticles := []models.Article{}
			for _, a := range feedArticles {
				if !a.Published.After(cutoff) {
					continue
				}

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
				articles = append(articles, robinArticles...)
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

func fetchSingleFeed(ctx context.Context, source models.FeedSource, torProxy string) ([]models.Article, error) {
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

	feed, err := fp.ParseURLWithContext(source.URL, ctx)
	if err != nil {
		return nil, err
	}

	articles := []models.Article{}
	for _, item := range feed.Items {
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		if pubDate.After(time.Now().Add(36 * time.Hour)) {
			continue
		}

		desc := item.Description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}

		articles = append(articles, models.Article{
			Title:       item.Title,
			Link:        item.Link,
			Description: desc,
			Content:     item.Content,
			Published:   pubDate,
			SourceName:  source.Name,
		})
	}

	return articles, nil
}
