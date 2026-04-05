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

type FeedSource struct {
	Name	string
	URL	string
}

type Article struct {
	Title		string
	Link		string
	Description	string
	Content		string	// Raw content for extraction
	Published	time.Time
	SourceName	string
	Score		int
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

type FetchResult struct {
	Articles	[]Article
	TotalFeeds	int
	FetchedFeeds	int
	Duration	time.Duration
}

var (
	advisoryPattern	= regexp.MustCompile(`(?i)^(CVE-\d|ZDI-\d|[A-Z]+-SA-|RHSA-|DSA-|USN-|GHSA-)`)
	cvePattern	= regexp.MustCompile(`(?i)CVE-\d{4}-\d+`)
)

func FetchAll(ctx context.Context, cfg *AppConfig, db *IntelligenceDB) (FetchResult, error) {
	start := time.Now()
	feeds, err := LoadFeeds()
	if err != nil {
		return FetchResult{}, err
	}

	extractor := NewExtractor()
	var (
		articles	[]Article
		mu		sync.Mutex
		fetched		int
	)

	g, ctx := errgroup.WithContext(ctx)

	g.SetLimit(500)

	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	for _, src := range feeds {
		src := src
		g.Go(func() error {
			feedArticles, err := fetchSingleFeed(ctx, src, cfg)
			if err != nil {
				return nil
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

	g.Go(func() error {
		dragnetArticles := FetchDragnetFeeds(ctx, cfg)
		var validArticles []Article
		for _, a := range dragnetArticles {
			ScoreArticle(&a, cfg)
			validArticles = append(validArticles, a)

			if db != nil {
				ents := extractor.ExtractEntities(a)
				_ = db.SaveArticle(a, ents)
			}
		}
		mu.Lock()
		articles = append(articles, validArticles...)
		mu.Unlock()
		return nil
	})

	_ = g.Wait()

	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterArticles(articles)

	finalArticles := []Article{}
	for _, c := range clusters {
		finalArticles = append(finalArticles, c.PrimaryArticle)
	}

	sort.Slice(finalArticles, func(i, j int) bool {
		if finalArticles[i].Score == finalArticles[j].Score {
			return finalArticles[i].Published.After(finalArticles[j].Published)
		}
		return finalArticles[i].Score > finalArticles[j].Score
	})

	return FetchResult{
		Articles:	finalArticles,
		TotalFeeds:	len(feeds),
		FetchedFeeds:	fetched,
		Duration:	time.Since(start),
	}, nil
}

func fetchSingleFeed(ctx context.Context, source FeedSource, cfg *AppConfig) ([]Article, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	fp := gofeed.NewParser()
	fp.UserAgent = "Recon/2.0 (+https://github.com/recon-cli)"

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

		if pubDate.After(time.Now().Add(36 * time.Hour)) {
			continue
		}

		desc := item.Description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}

		articles = append(articles, Article{
			Title:		item.Title,
			Link:		item.Link,
			Description:	desc,
			Content:	item.Content,
			Published:	pubDate,
			SourceName:	source.Name,
		})
	}

	return articles, nil
}

func ScoreArticle(a *Article, cfg *AppConfig) {
	score := 0
	text := strings.ToLower(a.Title + " " + a.Description)

	if cfg != nil {
		for _, kw := range cfg.Keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				score += 3
			}
		}
	}

	if advisoryPattern.MatchString(a.Title) {
		score -= 30
	}

	if cvePattern.MatchString(a.Title) {
		score += 15
	}

	if strings.Contains(text, "how i") || strings.Contains(text, "deep dive") || strings.Contains(text, "lessons learned") || strings.Contains(text, "internals of") {
		score += 15
	}

	narrativeKeys := []string{"root cause", "rca", "timeline", "chain of events", "ttps", "mitre att&ck", "forensic", "methodology", "attribution", "uncovering", "detailed analysis"}
	for _, k := range narrativeKeys {
		if strings.Contains(text, k) {
			score += 10
		}
	}

	if cvePattern.MatchString(a.Title) {
		isNarrative := false
		for _, k := range narrativeKeys {
			if strings.Contains(text, k) {
				isNarrative = true
				break
			}
		}
		if isNarrative {
			score += 20
		}
	}

	if len(a.Title) > 60 {
		score += 5
	}

	if HighValueSources[a.SourceName] {
		score += 50
	}

	if strings.Contains(text, "zero-day") || strings.Contains(text, "0day") {
		score += 5
	}

	lowSignalDomains := []string{"medium.com", "dev.to", "hashnode.com"}
	for _, d := range lowSignalDomains {
		if strings.Contains(strings.ToLower(a.Link), d) {
			score -= 25
		}
	}

	fluffKeys := []string{"fresher", "roadmap", "career", "interview", "salary", "beginner guide", "top 10", "how to start", "prompt engineering"}
	for _, k := range fluffKeys {
		if strings.Contains(text, k) {
			score -= 40
		}
	}

	a.Score = score
}

var HighValueSources = map[string]bool{
	"Simon Willison":		true,
	"George Hotz (geohot)":		true,
	"Julia Evans (jvns)":		true,
	"Dan Luu":			true,
	"Filippo Valsorda":		true,
	"Tavis Ormandy":		true,
	"Qualys Threat Research":	true,
	"Rapid7 Blog":			true,
	"CrowdStrike":			true,
	"Palo Alto Unit 42":		true,
	"Mandiant (Google Cloud)":	true,
	"Cisco Talos":			true,
	"Krebs on Security":		true,
	"Phoronix (Linux)":		true,
	"The Hacker News":		true,
	"Elastic Security Labs":	true,
	"Palo Alto Networks":		true,
	"Check Point Research":		true,
	"BleepingComputer":		true,
	"The Register (Security)":	true,
}
