package fetcher

import (
	"encoding/json"
	"fmt"
	"news-cli/internal/models"
	"news-cli/internal/scorer"
	"os"
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

func IngestRobinIntel(filePath string) ([]models.Article, error) {
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
		score := 50 // Base score for dark web hits
		if item.Severity == "critical" {
			score += 50
		}

		art := models.Article{
			Title:       item.Title,
			Link:        item.URL,
			Description: item.Content,
			Published:   intel.FetchTime,
			SourceName:  fmt.Sprintf("[DARKWEB:%s]", item.Severity),
			Score:       score,
		}

		// Re-score with CVE patterns if applicable
		scorer.ScoreArticle(&art, nil)
		articles = append(articles, art)
	}

	return articles, nil
}
