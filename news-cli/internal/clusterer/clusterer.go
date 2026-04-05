package clusterer

import (
	"news-cli/internal/models"
	"strings"
)

type Clusterer struct {
	MaxHammingDistance int
	sh                 *SimHash
}

func NewClusterer(threshold int) *Clusterer {
	return &Clusterer{
		MaxHammingDistance: threshold,
		sh:                 NewSimHash(),
	}
}

func (c *Clusterer) ClusterArticles(articles []models.Article) []models.ClusterGroup {
	var clusters []models.ClusterGroup
	fps := make(map[string]uint64)

	for _, art := range articles {
		h := art.Hash()
		fp := c.sh.Fingerprint(art.Title + " " + art.Description)
		fps[h] = fp

		found := false
		for i, group := range clusters {
			primaryH := group.PrimaryArticle.Hash()
			if HammingDistance(fp, fps[primaryH]) <= c.MaxHammingDistance {
				clusters[i].RelatedArticles = append(clusters[i].RelatedArticles, art)
				found = true
				break
			}
		}

		if !found {
			clusters = append(clusters, models.ClusterGroup{
				ID:             h,
				PrimaryArticle: art,
			})
		}
	}

	return clusters
}
