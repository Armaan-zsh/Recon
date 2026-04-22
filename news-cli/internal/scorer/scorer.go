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

var ResearchSources = map[string]int{
	"Simon Willison":              45,
	"George Hotz (geohot)":        35,
	"Julia Evans (jvns)":          45,
	"Dan Luu":                     40,
	"Filippo Valsorda":            45,
	"Tavis Ormandy":               45,
	"Qualys Threat Research":      40,
	"Rapid7 Blog":                 35,
	"CrowdStrike":                 28,
	"www.crowdstrike.com/blog":    28,
	"Palo Alto Unit 42":           38,
	"Mandiant (Google Cloud)":     38,
	"Cisco Talos":                 38,
	"Krebs on Security":           35,
	"Elastic Security Labs":       35,
	"Palo Alto Networks":          30,
	"Check Point Research":        35,
	"Google Project Zero":         50,
	"Project Zero":                50,
	"Trail of Bits":               45,
	"The DFIR Report":             50,
	"Microsoft Security Blog":     32,
	"SentinelOne Labs":            36,
	"Huntress Blog":               34,
	"Google Online Security Blog": 36,
	"Cloudflare Blog (Security)":  30,
	"OpenAI Blog":                 24,
	"Red Canary":                  30,
	"The Red Canary Blog: Information Security Insights": 30,
}

var NewsDeskSources = map[string]int{
	"BleepingComputer":                     12,
	"The Register (Security)":              12,
	"The Hacker News":                      8,
	"SecurityWeek":                         10,
	"The Record from Recorded Future News": 12,
	"Phoronix (Linux)":                     8,
}

var LowSignalSources = map[string]int{
	"InfoSec Write-ups - Medium":                45,
	"Bug Bounty in InfoSec Write-ups on Medium": 50,
	"Have I been pwned? latest breaches":        40,
	"Security Affairs":                          25,
	"VentureBeat":                               15,
	"TechCrunch":                                18,
	"Wired":                                     18,
	"Cybersecurity News":                        90,
	"VulDB Recent Entries":                      140,
	"CXSecurity: World Laboratory of Bugtraq 2": 140,
	"defend.network":                            90,
}

func ScoreArticle(a *models.Article, keywords []string) {
	score := 0
	text := strings.ToLower(a.Title + " " + a.Description)
	keywordHits := 0
	signalHits := 0
	hasNarrative := false

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
			hasNarrative = true
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
			hasNarrative = true
		}
	}

	if len(a.Title) > 60 {
		score += 5
	}

	blogKeys := []string{
		"research", "deep dive", "analysis", "write-up", "writeup", "walkthrough",
		"postmortem", "incident report", "reverse engineering", "lessons learned",
		"root cause", "case study", "forensic", "investigation", "showed us",
	}
	for _, k := range blogKeys {
		if strings.Contains(text, k) {
			score += 12
			signalHits++
		}
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

	if bonus, ok := ResearchSources[a.SourceName]; ok && (keywordHits > 0 || signalHits > 0) {
		score += bonus
	}
	if bonus, ok := NewsDeskSources[a.SourceName]; ok && (keywordHits > 0 || signalHits > 0) {
		score += bonus
	}

	loadOnce.Do(func() {
		_ = LoadIntel()
	})

	cves := CvePattern.FindAllString(strings.ToUpper(a.Title+" "+a.Description), -1)
	if hasNarrative || !AdvisoryPattern.MatchString(a.Title) {
		for _, cve := range cves {
			score += GetKEVScoreBoost(cve)
			score += GetEPSSScoreBoost(cve)
		}
	}

	if strings.Contains(text, "zero-day") || strings.Contains(text, "0day") {
		score += 5
		signalHits++
	}

	lowSignalDomains := []string{"medium.com", "dev.to", "hashnode.com", "cxsecurity.com", "vuldb.com", "cybersecuritynews.com", "defend.network"}
	for _, d := range lowSignalDomains {
		if strings.Contains(strings.ToLower(a.Link), d) {
			score -= 40
		}
	}

	// Reddit and social thread reposts are noisy — hard penalty.
	link := strings.ToLower(a.Link)
	source := strings.ToLower(a.SourceName)
	if strings.Contains(link, "reddit.com/") || strings.Contains(link, "redd.it/") || strings.Contains(source, "reddit") {
		score -= 200
	}

	fluffKeys := []string{"fresher", "roadmap", "career", "interview", "salary", "beginner guide", "top 10", "how to start", "prompt engineering"}
	for _, k := range fluffKeys {
		if strings.Contains(text, k) {
			score -= 40
		}
	}

	newsletterKeys := []string{"newsletter", "roundup", "round ", "latest breaches", "weekly", "daily digest", "digest", "patch tuesday", "security affairs newsletter", "daily threat briefing", "threat briefing", "threat report"}
	for _, k := range newsletterKeys {
		if strings.Contains(text, k) {
			score -= 35
		}
	}

	if strings.Contains(text, "threat intelligence report") || strings.Contains(text, "intel report") {
		score -= 45
	}

	advisoryKeys := []string{"security advisory", "advisory", "patches", "released updates", "updates available", "hotfix"}
	for _, k := range advisoryKeys {
		if strings.Contains(text, k) {
			score -= 18
		}
	}

	if AdvisoryPattern.MatchString(a.Title) && !hasNarrative {
		score -= 80
	}

	for source, penalty := range LowSignalSources {
		if a.SourceName == source {
			score -= penalty
			break
		}
	}

	if keywordHits == 0 && signalHits == 0 {
		score -= 45
	}

	a.Score = score
}
