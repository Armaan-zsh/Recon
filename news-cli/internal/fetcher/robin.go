package fetcher

import (
	"encoding/json"
	"fmt"
	"net/url"
	"news-cli/internal/models"
	"news-cli/internal/scorer"
	"os"
	"strings"
	"time"
)

type RobinIntel struct {
	Source    string      `json:"source"`
	FetchTime time.Time   `json:"fetch_time"`
	Items     []RobinItem `json:"items"`
}

type RobinItem struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	URL      string   `json:"url"`
	Tags     []string `json:"tags"`
	Severity string   `json:"severity"`
}

func IngestRobinIntel(filePath string, keywords []string, techStack []string, allowOnion bool) ([]models.Article, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read robin intel: %w", err)
	}

	var intel RobinIntel
	if err := json.Unmarshal(data, &intel); err != nil {
		return nil, fmt.Errorf("failed to parse robin intel: %w", err)
	}

	articles := make([]models.Article, 0, len(intel.Items))
	for _, item := range intel.Items {
		link, isOnion, ok := normalizeRobinURL(item.URL)
		if !ok {
			continue
		}
		if isOnion && !allowOnion {
			continue
		}
		sev := normalizeSeverity(item.Severity)
		conf := confidenceScore(sev, item.Tags, item.Title, item.Content)

		art := models.Article{
			Title:       sanitizeField(item.Title, 220),
			Link:        link,
			Description: sanitizeField(item.Content, 500),
			Published:   intel.FetchTime,
			SourceName:  fmt.Sprintf("[DARKWEB:%s:%d]", strings.ToUpper(sev), conf),
		}

		// Score with global model, then add darkweb confidence/severity signal.
		scorer.ScoreArticle(&art, keywords, techStack)
		art.Score += darkwebScoreBoost(sev, conf)
		articles = append(articles, art)
	}

	return articles, nil
}

func normalizeRobinURL(raw string) (string, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false, false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false, false
	}
	host := strings.ToLower(u.Hostname())
	return u.String(), strings.HasSuffix(host, ".onion"), true
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func sanitizeField(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

func confidenceScore(severity string, tags []string, title string, content string) int {
	score := 35
	switch severity {
	case "critical":
		score += 30
	case "high":
		score += 22
	case "medium":
		score += 14
	default:
		score += 6
	}
	score += minInt(20, len(tags)*4)

	text := strings.ToLower(title + " " + content)
	if strings.Contains(text, "cve-") {
		score += 8
	}
	if strings.Contains(text, "ransomware") || strings.Contains(text, "leak") || strings.Contains(text, "exploit") {
		score += 8
	}
	if score > 100 {
		score = 100
	}
	return score
}

func darkwebScoreBoost(severity string, confidence int) int {
	sevBoost := map[string]int{
		"critical": 55,
		"high":     38,
		"medium":   22,
		"low":      12,
	}[severity]
	return sevBoost + confidence/2
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
