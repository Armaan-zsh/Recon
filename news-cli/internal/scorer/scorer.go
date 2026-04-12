package scorer

import (
	"news-cli/internal/models"
	"regexp"
	"strings"
	"sync"
)

var loadOnce sync.Once

var (
	AdvisoryPattern = regexp.MustCompile(`(?i)^(CVE-\d|ZDI-\d|[A-Z]+-SA-|RHSA-|DSA-|USN-|GHSA-)`)
	CvePattern      = regexp.MustCompile(`(?i)CVE-\d{4}-\d+`)
)

var HighValueSources = map[string]bool{
	"Simon Willison":           true,
	"George Hotz (geohot)":     true,
	"Julia Evans (jvns)":       true,
	"Dan Luu":                  true,
	"Filippo Valsorda":         true,
	"Tavis Ormandy":            true,
	"Qualys Threat Research":   true,
	"Rapid7 Blog":              true,
	"CrowdStrike":              true,
	"Palo Alto Unit 42":        true,
	"Mandiant (Google Cloud)":  true,
	"Cisco Talos":              true,
	"Krebs on Security":        true,
	"Phoronix (Linux)":         true,
	"The Hacker News":          true,
	"Elastic Security Labs":    true,
	"Palo Alto Networks":       true,
	"Check Point Research":     true,
	"BleepingComputer":         true,
	"The Register (Security)":  true,
}

func ScoreArticle(a *models.Article, keywords []string) {
	score := 0
	text := strings.ToLower(a.Title + " " + a.Description)
	keywordHits := 0
	signalHits := 0

	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			score += 3
			keywordHits++
		}
	}

	if AdvisoryPattern.MatchString(a.Title) {
		score -= 30
		signalHits++
	}

	if CvePattern.MatchString(a.Title) {
		score += 15
		signalHits++
	}

	if strings.Contains(text, "how i") || strings.Contains(text, "deep dive") || strings.Contains(text, "lessons learned") || strings.Contains(text, "internals of") {
		score += 15
		signalHits++
	}

	narrativeKeys := []string{"root cause", "rca", "timeline", "chain of events", "ttps", "mitre att&ck", "forensic", "methodology", "attribution", "uncovering", "detailed analysis"}
	for _, k := range narrativeKeys {
		if strings.Contains(text, k) {
			score += 10
			signalHits++
		}
	}

	if CvePattern.MatchString(a.Title) {
		isNarrative := false
		for _, k := range narrativeKeys {
			if strings.Contains(text, k) {
				isNarrative = true
				break
			}
		}
		if isNarrative {
			score += 20
			signalHits++
		}
	}

	if len(a.Title) > 60 {
		score += 5
	}

	topicKeys := []string{
		"security", "vulnerability", "exploit", "malware", "ransomware", "breach",
		"privacy", "surveillance", "cryptography", "encryption", "supply chain",
		"zero-day", "0day", "prompt injection", "llm", "artificial intelligence",
		"machine learning", "cloud", "kubernetes", "linux", "incident",
	}
	for _, k := range topicKeys {
		if strings.Contains(text, k) {
			signalHits++
		}
	}

	if HighValueSources[a.SourceName] {
		if keywordHits > 0 || signalHits > 0 {
			score += 50
		}
	}

	loadOnce.Do(func() {
		_ = LoadIntel()
	})

	cves := CvePattern.FindAllString(strings.ToUpper(a.Title+" "+a.Description), -1)
	for _, cve := range cves {
		score += GetKEVScoreBoost(cve)
		score += GetEPSSScoreBoost(cve)
	}

	if strings.Contains(text, "zero-day") || strings.Contains(text, "0day") {
		score += 5
		signalHits++
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

	if keywordHits == 0 && signalHits == 0 {
		score -= 45
	}

	a.Score = score
}
