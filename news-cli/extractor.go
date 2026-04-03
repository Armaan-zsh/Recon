package main

import (
	"regexp"
	"strings"
)

// EntityExtractor finds technical indicators and actors in text.
type EntityExtractor struct {
	cveRegex	*regexp.Regexp
	ipRegex		*regexp.Regexp
	domainRegex	*regexp.Regexp
	aptRegex	*regexp.Regexp
	malwareRefs	[]string
	aptNames	[]string
}

// NewExtractor initializes the regex patterns and entity lookup lists.
func NewExtractor() *EntityExtractor {
	return &EntityExtractor{
		cveRegex:	regexp.MustCompile(`(?i)CVE-\d{4}-\d{4,}`),
		ipRegex:	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		domainRegex:	regexp.MustCompile(`(?i)\b([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}\b`),
		aptRegex:	regexp.MustCompile(`(?i)\b(APT\d+|Lazarus|Volt Typhoon|Fancy Bear|Cozy Bear|Turla|Sandworm|Sandman|Stardust Chollima|Kimsuky)\b`),
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

// ExtractEntities parses an article for technical entities.
func (e *EntityExtractor) ExtractEntities(a Article) []string {
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
