package scorer

import (
	"math"
	"news-cli/internal/models"
	"strings"
	"time"
)

type LaneSignals struct {
	GeneralScore int
	ExpertScore  int
	GeneralWhy   []string
	ExpertWhy    []string
}

func ScoreLanes(a models.Article) LaneSignals {
	text := strings.ToLower(a.Title + " " + a.Description)
	ageHours := time.Since(a.Published).Hours()
	if ageHours < 0 {
		ageHours = 0
	}

	general := int(float64(a.Score) * 0.5)
	expert := a.Score
	var generalWhy []string
	var expertWhy []string

	recencyBoost := int(math.Max(0, 24-ageHours)) / 2
	general += recencyBoost
	expert += recencyBoost / 2
	if recencyBoost > 0 {
		generalWhy = append(generalWhy, "fresh update")
	}

	if CvePattern.MatchString(a.Title + " " + a.Description) {
		expert += 25
		expertWhy = append(expertWhy, "contains CVE")
	}

	if strings.Contains(text, "kev") || strings.Contains(text, "known exploited vulnerabilities") {
		expert += 20
		expertWhy = append(expertWhy, "known exploited context")
	}

	if containsAny(text, "exploited in the wild", "active exploitation", "zero-day", "0day", "rce", "remote code execution") {
		expert += 18
		expertWhy = append(expertWhy, "active exploitation signal")
	}

	if len(a.IoCs) > 0 {
		expert += 12
		expertWhy = append(expertWhy, "includes IOCs")
	}
	if strings.TrimSpace(a.PatchLink) != "" {
		expert += 10
		expertWhy = append(expertWhy, "patch available")
	}

	if containsAny(text, "what happened", "explained", "impact", "how to", "guide", "lessons learned") {
		general += 12
		generalWhy = append(generalWhy, "easy-to-digest context")
	}
	if containsAny(text, "breach", "ransomware", "critical", "emergency", "supply chain") {
		general += 10
		generalWhy = append(generalWhy, "high user impact")
	}

	if strings.Contains(strings.ToLower(a.SourceName), "hacker news frontpage") {
		general -= 8
	}
	if AdvisoryPattern.MatchString(a.Title) {
		general -= 20
		expert += 8
		expertWhy = append(expertWhy, "vendor advisory")
	}

	return LaneSignals{
		GeneralScore: general,
		ExpertScore:  expert,
		GeneralWhy:   dedupeStrings(generalWhy),
		ExpertWhy:    dedupeStrings(expertWhy),
	}
}

func containsAny(text string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
