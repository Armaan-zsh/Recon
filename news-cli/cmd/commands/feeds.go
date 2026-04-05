package commands

import (
	"encoding/json"
	"fmt"
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
	// Try current directory first
	cwd, err := os.Getwd()
	if err == nil {
		localPath := filepath.Join(cwd, "links.json")
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// Try config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine config directory: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "recon")
	feedsPath := filepath.Join(appConfigDir, "links.json")

	// Check if exists, if not create from embedded or default
	if _, err := os.Stat(feedsPath); err == nil {
		return feedsPath, nil
	}

	// Create directory if needed
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Return path even if doesn't exist yet (for add command)
	return feedsPath, nil
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
		url := args[1]

		feeds, err := loadFeeds()
		if err != nil {
			return err
		}

		// Check for duplicates
		for _, feed := range feeds.Links {
			if strings.EqualFold(feed.URL, url) {
				return fmt.Errorf("feed with URL '%s' already exists as '%s'", url, feed.Name)
			}
		}

		feeds.Links = append(feeds.Links, FeedEntry{Name: name, URL: url})

		if err := saveFeeds(feeds); err != nil {
			return err
		}

		fmt.Printf("✅ Added feed: %s (%s)\n", name, url)
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
