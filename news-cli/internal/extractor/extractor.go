package extractor

import (
	"news-cli/internal/models"
	"regexp"
	"strings"
)

type EntityExtractor struct {
	cveRegex    *regexp.Regexp
	ipRegex     *regexp.Regexp
	domainRegex *regexp.Regexp
	aptRegex    *regexp.Regexp
	malwareRefs []string
	aptNames    []string
}

func NewExtractor() *EntityExtractor {
	return &EntityExtractor{
		cveRegex:    regexp.MustCompile(`(?i)CVE-\d{4}-\d{4,}`),
		ipRegex:     regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		domainRegex: regexp.MustCompile(`(?i)\b([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}\b`),
		aptRegex:    regexp.MustCompile(`(?i)\b(APT\d+|Lazarus|Volt Typhoon|Fancy Bear|Cozy Bear|Turla|Sandworm|Sandman|Stardust Chollima|Kimsuky)\b`),
		malwareRefs: []string{
			"Glass Worm", "RedLine", "AgentTesla", "IcedID", "Cobalt Strike",
			"Metasploit", "LockBit", "BlackCat", "Ahti", "VoidLink", "DarkGate",
			"Lumma", "Vidar", "Snake", "Emotet", "TrickBot", "Dridex", "Rootkit",
		},
		aptNames: []string{
			"Lazarus", "Volt Typhoon", "Fancy Bear", "Cozy Bear", "Turla",
			"Sandworm", "UNC5534", "Mustang Panda", "Silent Librarian",
		},
	}
}

func (e *EntityExtractor) ExtractEntities(a models.Article) []string {
	found := make(map[string]bool)
	text := a.Title + " " + a.Description + " " + a.Content

	cves := e.cveRegex.FindAllString(text, -1)
	for _, c := range cves {
		found[strings.ToUpper(c)] = true
	}

	apts := e.aptRegex.FindAllString(text, -1)
	for _, a := range apts {
		found[strings.ToUpper(a)] = true
	}

	lowerText := strings.ToLower(text)
	for _, m := range e.malwareRefs {
		if strings.Contains(lowerText, strings.ToLower(m)) {
			found[strings.ToUpper(m)] = true
		}
	}

	for _, n := range e.aptNames {
		if strings.Contains(lowerText, strings.ToLower(n)) {
			found[strings.ToUpper(n)] = true
		}
	}

	entities := make([]string, 0, len(found))
	for k := range found {
		entities = append(entities, k)
	}
	return entities
}

func (e *EntityExtractor) ExtractIoCs(a models.Article) []string {
	found := make(map[string]bool)
	text := a.Description + " " + a.Content // Usually IoCs are in the body
	
	ips := e.ipRegex.FindAllString(text, -1)
	for _, ip := range ips {
		// Ignore common local IPs or broadcast
		if ip != "127.0.0.1" && ip != "0.0.0.0" && ip != "255.255.255.255" {
			found[ip] = true
		}
	}
	
	// Hashes: MD5, SHA1, SHA256
	hashRegex := regexp.MustCompile(`(?i)\b([a-f0-9]{32}|[a-f0-9]{40}|[a-f0-9]{64})\b`)
	hashes := hashRegex.FindAllString(text, -1)
	for _, h := range hashes {
		found[strings.ToLower(h)] = true
	}

	iocs := make([]string, 0, len(found))
	for k := range found {
		iocs = append(iocs, k)
	}
	return iocs
}

func (e *EntityExtractor) ExtractPatchLink(a models.Article) string {
	text := a.Description + " " + a.Content
	
	// Regex for GitHub/GitLab commit links
	patchRegex := regexp.MustCompile(`(?i)https?://(github|gitlab)\.com/[^/]+/[^/]+/(commit|pull)/[a-f0-9]+`)
	link := patchRegex.FindString(text)
	
	return link
}
