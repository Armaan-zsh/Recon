package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"news-cli/internal/config"
	"news-cli/internal/feeds"
	"news-cli/internal/models"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// FeedEntry represents a single feed in links.json
type FeedEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// FeedsData represents the structure of links.json
type FeedsData struct {
	Links []FeedEntry `json:"links"`
}

func getFeedsFilePath() (string, error) {
	path, err := feeds.Path()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return path, nil
}

func loadFeeds() (*FeedsData, error) {
	path, err := getFeedsFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FeedsData{Links: []FeedEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read feeds file: %w", err)
	}

	var feeds FeedsData
	if err := json.Unmarshal(data, &feeds); err != nil {
		return nil, fmt.Errorf("failed to parse feeds JSON: %w", err)
	}

	return &feeds, nil
}

func saveFeeds(feeds *FeedsData) error {
	path, err := getFeedsFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(feeds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal feeds: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

var feedsCmd = &cobra.Command{
	Use:   "feeds",
	Short: "Manage RSS feed sources",
	Long:  "Add, remove, list, and cleanup RSS feed sources for Recon.",
}

var feedKind string

var feedsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured feeds",
	RunE: func(cmd *cobra.Command, args []string) error {
		feeds, err := loadFeeds()
		if err != nil {
			return err
		}

		fmt.Printf("📡 Configured Feeds (%d total):\n\n", len(feeds.Links))
		for i, feed := range feeds.Links {
			fmt.Printf("%3d. %-40s\n    %s\n", i+1, feed.Name, feed.URL)
		}
		return nil
	},
}

var feedsAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Add a new feed source",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		rawURL := args[1]
		normalizedURL, err := normalizeFeedURL(rawURL)
		if err != nil {
			return err
		}
		if err := validateFeedURLSafety(normalizedURL, feedKind); err != nil {
			return err
		}
		name = formatFeedName(name, feedKind)

		feeds, err := loadFeeds()
		if err != nil {
			return err
		}

		// Check for duplicates
		for _, feed := range feeds.Links {
			if strings.EqualFold(feed.URL, normalizedURL) {
				return fmt.Errorf("feed with URL '%s' already exists as '%s'", normalizedURL, feed.Name)
			}
		}

		feeds.Links = append(feeds.Links, FeedEntry{Name: name, URL: normalizedURL})

		if err := saveFeeds(feeds); err != nil {
			return err
		}

		fmt.Printf("✅ Added feed: %s (%s)\n", name, normalizedURL)
		if feedKind == "youtube" && !strings.Contains(normalizedURL, "feeds/videos.xml") {
			fmt.Println("⚠️  YouTube URL added, but best reliability is channel RSS (feeds/videos.xml?channel_id=...).")
		}
		return nil
	},
}

var feedsRemoveCmd = &cobra.Command{
	Use:   "remove <url-or-pattern>",
	Short: "Remove feed(s) matching URL or pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := strings.ToLower(args[0])

		feeds, err := loadFeeds()
		if err != nil {
			return err
		}

		initialCount := len(feeds.Links)
		filtered := []FeedEntry{}

		for _, feed := range feeds.Links {
			if !strings.Contains(strings.ToLower(feed.URL), pattern) &&
				!strings.Contains(strings.ToLower(feed.Name), pattern) {
				filtered = append(filtered, feed)
			}
		}

		removed := initialCount - len(filtered)
		if removed == 0 {
			fmt.Printf("⚠️  No feeds matched pattern: %s\n", pattern)
			return nil
		}

		feeds.Links = filtered
		if err := saveFeeds(feeds); err != nil {
			return err
		}

		fmt.Printf("✅ Removed %d feed(s) matching: %s\n", removed, pattern)
		return nil
	},
}

var feedsCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove duplicate and invalid feeds",
	RunE: func(cmd *cobra.Command, args []string) error {
		feeds, err := loadFeeds()
		if err != nil {
			return err
		}

		initialCount := len(feeds.Links)
		seen := make(map[string]bool)
		cleaned := []FeedEntry{}

		for _, feed := range feeds.Links {
			urlLower := strings.ToLower(strings.TrimSpace(feed.URL))
			if !seen[urlLower] {
				seen[urlLower] = true
				cleaned = append(cleaned, feed)
			}
		}

		removed := initialCount - len(cleaned)
		if removed == 0 {
			fmt.Println("✅ No duplicates found. Feeds are clean!")
			return nil
		}

		feeds.Links = cleaned
		if err := saveFeeds(feeds); err != nil {
			return err
		}

		fmt.Printf("✅ Removed %d duplicate feed(s). Total feeds: %d\n", removed, len(feeds.Links))
		return nil
	},
}

var feedsMergeCmd = &cobra.Command{
	Use:   "merge <file.json>",
	Short: "Merge feeds from another JSON file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mergeFile := args[0]

		// Load existing feeds
		existing, err := loadFeeds()
		if err != nil {
			return err
		}

		// Load feeds to merge
		data, err := os.ReadFile(mergeFile)
		if err != nil {
			return fmt.Errorf("failed to read merge file: %w", err)
		}

		var toMerge FeedsData
		if err := json.Unmarshal(data, &toMerge); err != nil {
			return fmt.Errorf("failed to parse merge file: %w", err)
		}

		// Merge with deduplication
		existingURLs := make(map[string]bool)
		for _, feed := range existing.Links {
			existingURLs[strings.ToLower(feed.URL)] = true
		}

		added := 0
		for _, feed := range toMerge.Links {
			if !existingURLs[strings.ToLower(feed.URL)] {
				existing.Links = append(existing.Links, feed)
				added++
			}
		}

		if err := saveFeeds(existing); err != nil {
			return err
		}

		fmt.Printf("✅ Merged %d new feed(s). Total feeds: %d\n", added, len(existing.Links))
		return nil
	},
}

func init() {
	feedsAddCmd.Flags().StringVar(&feedKind, "kind", "blog", "Source kind: blog|youtube|forum|darkweb|custom")
	feedsCmd.AddCommand(feedsListCmd)
	feedsCmd.AddCommand(feedsAddCmd)
	feedsCmd.AddCommand(feedsRemoveCmd)
	feedsCmd.AddCommand(feedsCleanupCmd)
	feedsCmd.AddCommand(feedsMergeCmd)
}

// GetFeedsCmd returns the feeds command for use in main.go
func GetFeedsCmd() *cobra.Command {
	return feedsCmd
}

func normalizeFeedURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid URL: %s", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s (only http/https allowed)", u.Scheme)
	}
	return u.String(), nil
}

func validateFeedURLSafety(feedURL string, kind string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "blog", "youtube", "forum", "darkweb", "custom":
	default:
		return fmt.Errorf("invalid --kind '%s' (use blog|youtube|forum|darkweb|custom)", kind)
	}

	isOnion := models.IsOnionURL(feedURL)
	if kind == "darkweb" && !isOnion {
		return fmt.Errorf("darkweb feed must use a .onion URL")
	}
	if isOnion {
		cfg, err := config.LoadConfig()
		if err != nil || cfg == nil || strings.TrimSpace(cfg.TorProxy) == "" {
			return fmt.Errorf("onion feeds require configured tor_proxy in recon config")
		}
	}
	return nil
}

func formatFeedName(name string, kind string) string {
	name = strings.TrimSpace(name)
	kind = strings.ToUpper(strings.TrimSpace(kind))
	prefix := "[" + kind + "] "
	if strings.HasPrefix(strings.ToUpper(name), prefix) {
		return name
	}
	return prefix + name
}
