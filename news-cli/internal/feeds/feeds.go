package feeds

import (
	"encoding/json"
	"fmt"
	"news-cli/internal/models"
	"os"
	"path/filepath"
	"strings"
)

type filePayload struct {
	Links []models.FeedSource `json:"links"`
}

var curatedFeeds = []models.FeedSource{
	{Name: "Cloudflare Blog (Security)", URL: "https://blog.cloudflare.com/tag/security/rss/"},
	{Name: "Google Project Zero", URL: "https://googleprojectzero.blogspot.com/feeds/posts/default"},
	{Name: "Google Online Security Blog", URL: "https://security.googleblog.com/feeds/posts/default"},
	{Name: "Trail of Bits", URL: "https://blog.trailofbits.com/feed/"},
	{Name: "The DFIR Report", URL: "https://thedfirreport.com/feed/"},
	{Name: "Microsoft Security Blog", URL: "https://www.microsoft.com/security/blog/feed/"},
	{Name: "SentinelOne Labs", URL: "https://www.sentinelone.com/labs/feed/"},
	{Name: "Huntress Blog", URL: "https://www.huntress.com/blog/rss.xml"},
	{Name: "CrowdStrike", URL: "https://www.crowdstrike.com/en-us/blog/feed/"},
	{Name: "Red Canary", URL: "https://www.redcanary.com/blog/feed/"},
	{Name: "OpenAI Blog", URL: "https://openai.com/blog/rss.xml"},
}

func Path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine config directory: %w", err)
	}

	return filepath.Join(configDir, "recon", "links.json"), nil
}

func LoadData(defaultData []byte) ([]byte, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read feeds file: %w", readErr)
		}
		merged, changed, err := mergeCuratedFeeds(data)
		if err != nil {
			return nil, fmt.Errorf("invalid feeds file %s: %w", path, err)
		}
		if changed {
			if err := os.WriteFile(path, merged, 0644); err != nil {
				return nil, fmt.Errorf("failed to update feeds file: %w", err)
			}
		}
		return merged, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat feeds file: %w", err)
	}

	if len(defaultData) == 0 {
		return []byte(`{"links":[]}`), nil
	}

	if err := Validate(defaultData); err != nil {
		return nil, fmt.Errorf("embedded feeds are invalid: %w", err)
	}

	merged, _, err := mergeCuratedFeeds(defaultData)
	if err != nil {
		return nil, fmt.Errorf("failed to merge curated feeds: %w", err)
	}

	if err := os.WriteFile(path, merged, 0644); err != nil {
		return nil, fmt.Errorf("failed to seed feeds file: %w", err)
	}

	return merged, nil
}

func Validate(data []byte) error {
	var payload filePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	return nil
}

func mergeCuratedFeeds(data []byte) ([]byte, bool, error) {
	var payload filePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, false, err
	}

	seen := make(map[string]bool, len(payload.Links))
	for _, feed := range payload.Links {
		seen[normalizeURL(feed.URL)] = true
	}

	changed := false
	for _, feed := range curatedFeeds {
		key := normalizeURL(feed.URL)
		if seen[key] {
			continue
		}
		payload.Links = append(payload.Links, feed)
		seen[key] = true
		changed = true
	}

	merged, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return merged, changed, nil
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	return strings.TrimRight(s, "/")
}
