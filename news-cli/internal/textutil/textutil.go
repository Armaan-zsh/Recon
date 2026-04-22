package textutil

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagPattern        = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespacePattern = regexp.MustCompile(`\s+`)
)

func PlainText(s string) string {
	s = html.UnescapeString(s)
	s = tagPattern.ReplaceAllString(s, " ")
	s = whitespacePattern.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func Truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}
