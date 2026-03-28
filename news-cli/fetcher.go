package main

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
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
	Published   time.Time
	SourceName  string
	Score       int
}

// HighValueSources are known authoritative vulnerability research sources.
// Articles from these sources get a bonus score.
var HighValueSources = map[string]bool{
	"Simon Willison":              true,
	"George Hotz (geohot)":        true,
	"Julia Evans (jvns)":          true,
	"Dan Luu":                     true,
	"Filippo Valsorda":            true,
	"Hanno Böck":                  true,
	"Tavis Ormandy":               true,
	"The Grugq":                   true,
	"Qualys Threat Research":      true,
	"Rapid7 Blog":                 true,
	"CrowdStrike":                 true,
	"Palo Alto Unit 42":           true,
	"Mandiant (Google Cloud)":     true,
	"Zero Day Initiative":         true,
	"Cisco Talos":                 true,
	"Microsoft MSRC":              true,
	"SANS ISC":                    true,
	"Google Project Zero":         true,
	"Trail of Bits":               true,
	"Krebs on Security":           true,
	"Schneier on Security":        true,
	"Elastic Security Labs":       true,
	"SentinelOne Labs":            true,
	"Check Point Research":        true,
	"PortSwigger Research":        true,
	"CISA Alerts":                 true,
	"Signal Blog":                 true,
	"Phoronix (Linux)":            true,
	"The Register (Security)":     true,
	"BleepingComputer":            true,
	"The Hacker News":             true,
	"Platformer":                  true,
}

// AntiKeywords cause an article to be immediately scored 0.
// These filter out real-world politics / physical security noise.
var AntiKeywords = []string{
	"police", " ice ", "border patrol", "law enforcement",
	"physical security", "guard ", "officer", "immigrat",
	"deportat", "customs", "border wall",
}

var cvePattern = regexp.MustCompile(`(?i)CVE-\d{4}-\d+`)

// ScoreArticle applies multi-signal scoring to an article.
func ScoreArticle(a *Article, keywords []string) {
	title := strings.ToLower(a.Title)
	desc := strings.ToLower(a.Description)
	text := title + " " + desc

	// Anti-keyword check: immediate disqualification
	for _, ak := range AntiKeywords {
		if strings.Contains(text, ak) {
			a.Score = 0
			return
		}
	}

	score := 0

	// Keyword matching (title weighted heavier than description)
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if strings.Contains(title, kwLower) {
			score += 10 // Title match = high signal
		} else if strings.Contains(desc, kwLower) {
			score += 5 // Description match = moderate signal
		}
	}

	// CVE pattern bonus
	if cvePattern.MatchString(a.Title) || cvePattern.MatchString(a.Description) {
		score += 8
	}

	// Zero-day bonus
	if strings.Contains(text, "zero-day") || strings.Contains(text, "0day") || strings.Contains(text, "zero day") {
		score += 10
	}

	// Breach/leak bonus
	if strings.Contains(text, "breach") || strings.Contains(text, "leak") || strings.Contains(text, "data exposure") {
		score += 8
	}

	// Critical severity bonus
	if strings.Contains(text, "critical") || strings.Contains(text, "remote code execution") || strings.Contains(text, "rce") {
		score += 7
	}

	// High-value source (Researcher) bonus
	if HighValueSources[a.SourceName] {
		score += 10
	}

	a.Score = score
}

// FetchResult holds the results from fetching all feeds.
type FetchResult struct {
	Articles     []Article
	TotalFeeds   int
	FetchedFeeds int
	Duration     time.Duration
}

// FetchFeeds concurrently fetches all provided feed sources using errgroup
// with bounded concurrency. It filters by recency, deduplicates, and scores.
func FetchFeeds(ctx context.Context, sources []FeedSource, keywords []string, strictFilter bool, cfg *AppConfig) FetchResult {
	start := time.Now()
	// Expanded to 7-day cutoff for weekly research deep-dives
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	var (
		articles   []Article
		mu         sync.Mutex
		seenTitles = make(map[string]bool)
		fetched    int
	)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(50) // Max 50 concurrent HTTP requests to prevent DNS/TCP starvation

	for _, src := range sources {
		src := src // Pin for closure
		g.Go(func() error {
			fp := gofeed.NewParser()
			fp.UserAgent = "Recon/2.0 (+https://github.com/recon-cli)"

			// Support Tor proxy for .onion feeds
			if strings.HasSuffix(strings.Split(src.URL, "/")[2], ".onion") && cfg != nil && cfg.TorProxy != "" {
				proxyURL, err := url.Parse(cfg.TorProxy)
				if err == nil {
					dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
					if err == nil {
						fp.Client = &http.Client{
							Transport: &http.Transport{
								Dial: dialer.Dial,
							},
							Timeout: 30 * time.Second, // Onions are slow
						}
					}
				}
			}

			// Reasonable 10s timeout per feed
			fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			feed, err := fp.ParseURLWithContext(src.URL, fetchCtx)
			if err != nil {
				return nil // Silently skip broken feeds, don't abort the group
			}

			mu.Lock()
			fetched++
			mu.Unlock()

			var feedArticles []Article
			for _, item := range feed.Items {
				pubTime := item.PublishedParsed
				if pubTime == nil {
					pubTime = item.UpdatedParsed
				}
				if pubTime == nil {
					now := time.Now()
					pubTime = &now
				}

				if !pubTime.After(cutoff) {
					continue
				}

				desc := item.Description
				if len(desc) > 300 {
					desc = desc[:300] + "..."
				}

				normalizedTitle := strings.ToLower(strings.TrimSpace(item.Title))

				mu.Lock()
				if seenTitles[normalizedTitle] {
					mu.Unlock()
					continue
				}
				seenTitles[normalizedTitle] = true
				mu.Unlock()

				a := Article{
					Title:       item.Title,
					Link:        item.Link,
					Description: desc,
					Published:   *pubTime,
					SourceName:  src.Name,
				}
				ScoreArticle(&a, keywords)

				if strictFilter && a.Score <= 0 {
					continue
				}

				feedArticles = append(feedArticles, a)
			}

			mu.Lock()
			articles = append(articles, feedArticles...)
			mu.Unlock()

			return nil
		})
	}

	_ = g.Wait() // We don't propagate errors since we silently skip broken feeds

	return FetchResult{
		Articles:     articles,
		TotalFeeds:   len(sources),
		FetchedFeeds: fetched,
		Duration:     time.Since(start),
	}
}
