package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/errgroup"
)

// FeedSource represents a single RSS/Atom feed to fetch.
type FeedSource struct {
	Name string
	URL  string
}

// Article represents a single scored news item.
type Article struct {
	Title       string
	Link        string
	Description string
	Content     string // Raw content for extraction
	Published   time.Time
	SourceName  string
	Score       int
}

// Hash returns a unique SHA256 identifier for the article based on its link.
func (a Article) Hash() string {
	h := sha256.New()
	h.Write([]byte(a.Link))
	return hex.EncodeToString(h.Sum(nil))
}

// FetchResult holds the results from fetching all feeds.
type FetchResult struct {
	Articles     []Article
	TotalFeeds   int
	FetchedFeeds int
	Duration     time.Duration
}

// Global patterns for scoring
var (
	advisoryPattern = regexp.MustCompile(`(?i)^(CVE-\d|ZDI-\d|[A-Z]+-SA-|RHSA-|DSA-|USN-|GHSA-)`)
	cvePattern      = regexp.MustCompile(`(?i)CVE-\d{4}-\d+`)
)

// FetchAll synchronizes all feeds concurrently and updates the database.
func FetchAll(ctx context.Context, cfg *AppConfig, db *IntelligenceDB) (FetchResult, error) {
	start := time.Now()
	feeds, err := LoadFeeds()
	if err != nil {
		return FetchResult{}, err
	}

	extractor := NewExtractor()
	var (
		articles []Article
		mu       sync.Mutex
		fetched  int
	)

	g, ctx := errgroup.WithContext(ctx)
	// Scaling to 500 workers for the 1,900+ feed Motherlode
	g.SetLimit(500)

	// cutoff for "Freshness" logic (7 days)
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	for _, src := range feeds {
		src := src
		g.Go(func() error {
			feedArticles, err := fetchSingleFeed(ctx, src, cfg)
			if err != nil {
				return nil // Skip broken feeds
			}

			mu.Lock()
			fetched++
			mu.Unlock()

			validArticles := []Article{}
			for _, a := range feedArticles {
				if !a.Published.After(cutoff) {
					continue
				}

				ScoreArticle(&a, cfg)
				if a.Score > 5 {
					validArticles = append(validArticles, a)
					
					// Real-time Intelligence Extraction and Persistence
					if db != nil {
						ents := extractor.ExtractEntities(a)
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

	_ = g.Wait()

	// De-duplicate in memory for the TUI display
	clusterer := NewClusterer(0.85) // High threshold for deduplication
	clusters := clusterer.ClusterArticles(articles)
	
	finalArticles := []Article{}
	for _, c := range clusters {
		finalArticles = append(finalArticles, c.PrimaryArticle)
	}

	// Sort by Score desc, then Date desc
	sort.Slice(finalArticles, func(i, j int) bool {
		if finalArticles[i].Score == finalArticles[j].Score {
			return finalArticles[i].Published.After(finalArticles[j].Published)
		}
		return finalArticles[i].Score > finalArticles[j].Score
	})

	return FetchResult{
		Articles:     finalArticles,
		TotalFeeds:   len(feeds),
		FetchedFeeds: fetched,
		Duration:     time.Since(start),
	}, nil
}

func fetchSingleFeed(ctx context.Context, source FeedSource, cfg *AppConfig) ([]Article, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	fp := gofeed.NewParser()
	fp.UserAgent = "Recon/2.0 (+https://github.com/recon-cli)"

	// Support Tor proxy for .onion feeds
	if strings.HasSuffix(strings.Split(source.URL, "/")[2], ".onion") && cfg != nil && cfg.TorProxy != "" {
		proxyURL, err := url.Parse(cfg.TorProxy)
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

	articles := []Article{}
	for _, item := range feed.Items {
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		desc := item.Description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}

		articles = append(articles, Article{
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

func ScoreArticle(a *Article, cfg *AppConfig) {
	score := 0
	text := strings.ToLower(a.Title + " " + a.Description)

	// Principal keyword scoring
	if cfg != nil {
		for _, kw := range cfg.Keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				score += 3
			}
		}
	}

	// Double penalty for robotic advisory titles: -30
	if advisoryPattern.MatchString(a.Title) {
		score -= 30
	}

	// CVE ID presence bonus
	if cvePattern.MatchString(a.Title) {
		score += 15
	}

	// Expert blog quality bonuses
	if strings.Contains(text, "how i") || strings.Contains(text, "deep dive") || strings.Contains(text, "lessons learned") || strings.Contains(text, "internals of") {
		score += 12
	}

	// Narrative title length bonus
	if len(a.Title) > 60 {
		score += 5
	}

	// High-value source (Researcher/GOAT) bonus
	if HighValueSources[a.SourceName] {
		score += 25
	}

	// Zero-day detections
	if strings.Contains(text, "zero-day") || strings.Contains(text, "0day") {
		score += 5
	}

	a.Score = score
}

// HighValueSources are known authoritative technical sources.
var HighValueSources = map[string]bool{
	"Simon Willison":          true,
	"George Hotz (geohot)":    true,
	"Julia Evans (jvns)":      true,
	"Dan Luu":                 true,
	"Filippo Valsorda":        true,
	"Tavis Ormandy":           true,
	"Qualys Threat Research":  true,
	"Rapid7 Blog":             true,
	"CrowdStrike":             true,
	"Palo Alto Unit 42":       true,
	"Mandiant (Google Cloud)": true,
	"Cisco Talos":             true,
	"Krebs on Security":       true,
	"Phoronix (Linux)":        true,
	"The Hacker News":         true,
	"Elastic Security Labs":   true,
	"Palo Alto Networks":      true,
	"Check Point Research":    true,
	"BleepingComputer":        true,
	"The Register (Security)": true,
}
