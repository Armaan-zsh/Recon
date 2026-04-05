package clusterer

import (
	"news-cli/internal/models"
	"strings"
)

type Clusterer struct {
	Threshold float64
}

func NewClusterer(threshold float64) *Clusterer {
	return &Clusterer{Threshold: threshold}
}

func (c *Clusterer) JaccardSimilarity(s1, s2 string) float64 {
	f1 := strings.Fields(strings.ToLower(s1))
	f2 := strings.Fields(strings.ToLower(s2))

	if len(f1) == 0 || len(f2) == 0 {
		return 0
	}

	m1 := make(map[string]bool)
	for _, f := range f1 {
		m1[f] = true
	}

	intersection := 0
	for _, f := range f2 {
		if m1[f] {
			intersection++
		}
	}

	union := len(f1) + len(f2) - intersection
	return float64(intersection) / float64(union)
}

func (c *Clusterer) ClusterArticles(articles []models.Article) []models.ClusterGroup {
	var clusters []models.ClusterGroup

	for _, art := range articles {
		found := false
		for i, group := range clusters {
			if c.JaccardSimilarity(art.Title, group.PrimaryArticle.Title) > c.Threshold {
				clusters[i].RelatedArticles = append(clusters[i].RelatedArticles, art)
				found = true
				break
			}
		}

		if !found {
			clusters = append(clusters, models.ClusterGroup{
				ID:             art.Hash(),
				PrimaryArticle: art,
			})
		}
	}

	return clusters
}
