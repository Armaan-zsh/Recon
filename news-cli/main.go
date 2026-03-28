package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed links.json
var linksData []byte

// LinksConfig is the raw structure of the embedded links.json dataset.
type LinksConfig struct {
	StackExchangeTags []string `json:"stack_exchange_tags"`
	GithubRepos       []string `json:"github_repos"`
	Subreddits        []string `json:"subreddits"`
	EngineeringBlogs  []struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"engineering_blogs"`
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "recon",
		Short: "Recon — Daily tech & cybersecurity intelligence",
		Long:  "Fetches, scores, and surfaces the best tech and cybersecurity news from 570+ feeds.",
		RunE:  runDefault,
	}

	rootCmd.Flags().Bool("browser", false, "Open results in browser sidebar view instead of TUI")
	rootCmd.Flags().StringSlice("tags", nil, "Override keywords for this run (e.g. --tags vulnerability,leak)")
	rootCmd.Flags().Bool("json", false, "Output results as JSON to stdout")

	// Init subcommand
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Run the setup wizard to configure preferences",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := RunSetupWizard()
			return err
		},
	}

	// Schedule subcommand
	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage the daily auto-run schedule",
	}

	scheduleSetCmd := &cobra.Command{
		Use:   "set",
		Short: "Set the auto-run time (e.g. recon schedule set --time 07:00)",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, _ := cmd.Flags().GetString("time")
			if t == "" {
				return fmt.Errorf("please provide --time flag (e.g. --time 07:00)")
			}
			fmt.Printf("Installing systemd timer for %s...\n", t)
			if err := ScheduleInstall(t); err != nil {
				return fmt.Errorf("failed to install schedule: %w", err)
			}
			// Update config
			cfg, _ := LoadConfig()
			if cfg != nil {
				cfg.ScheduleTime = t
				_ = SaveConfig(cfg)
			}
			fmt.Println("✓ Schedule set successfully.")
			return nil
		},
	}
	scheduleSetCmd.Flags().String("time", "", "Time in 24h format (e.g. 07:00)")

	scheduleDisableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable the auto-run schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ScheduleDisable()
		},
	}

	scheduleStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the auto-run schedule status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ScheduleStatus()
		},
	}

	scheduleCmd.AddCommand(scheduleSetCmd, scheduleDisableCmd, scheduleStatusCmd)
	rootCmd.AddCommand(initCmd, scheduleCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDefault(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// First-run check
	if cfg == nil || !cfg.SetupComplete {
		fmt.Println("First time running Recon! Let's set things up.")
		cfg, err = RunSetupWizard()
		if err != nil {
			return err
		}
	}

	// Missed run warning
	if MissedRun(cfg) {
		fmt.Println("⚠ You missed your last scheduled digest! Fetching now...")
	}

	// Determine keywords
	keywords := cfg.Keywords
	overrideTags, _ := cmd.Flags().GetStringSlice("tags")
	strictFilter := true

	if len(overrideTags) > 0 {
		keywords = overrideTags
		log.Printf("Using override keywords: %v", keywords)
	} else if len(args) > 0 {
		// Support: recon vulnerability,leak (positional arg)
		for _, arg := range args {
			for _, t := range strings.Split(arg, ",") {
				if s := strings.TrimSpace(t); s != "" {
					keywords = append(keywords, s)
				}
			}
		}
	}

	if len(keywords) == 0 {
		// No keywords at all, grab everything
		strictFilter = false
	}

	// Load the embedded feed database
	var feedsConfig LinksConfig
	if err := json.Unmarshal(linksData, &feedsConfig); err != nil {
		return fmt.Errorf("failed to parse feed database: %w", err)
	}

	var sources []FeedSource
	for _, blog := range feedsConfig.EngineeringBlogs {
		sources = append(sources, FeedSource{Name: blog.Name, URL: blog.Url})
	}

	log.Printf("Fetching %d feeds with keywords %v...", len(sources), keywords)

	// Fetch
	result := FetchFeeds(context.Background(), sources, keywords, strictFilter)

	// Sort: highest score first, then newest
	sort.Slice(result.Articles, func(i, j int) bool {
		if result.Articles[i].Score == result.Articles[j].Score {
			return result.Articles[i].Published.After(result.Articles[j].Published)
		}
		return result.Articles[i].Score > result.Articles[j].Score
	})

	// Cap at 50
	topN := 50
	if len(result.Articles) < topN {
		topN = len(result.Articles)
	}
	topArticles := result.Articles[:topN]

	log.Printf("Fetched %d articles from %d/%d feeds in %.1fs",
		len(topArticles), result.FetchedFeeds, result.TotalFeeds, result.Duration.Seconds())

	// Record last run
	_ = RecordLastRun(cfg)

	// Output mode
	jsonMode, _ := cmd.Flags().GetBool("json")
	browserMode, _ := cmd.Flags().GetBool("browser")

	if jsonMode {
		data, _ := json.MarshalIndent(topArticles, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if browserMode {
		htmlContent, err := renderHTML(topArticles)
		if err != nil {
			return fmt.Errorf("failed to generate HTML: %w", err)
		}
		serveAndOpen(htmlContent)
		return nil
	}

	// Default: TUI mode
	return runTUI(topArticles, result)
}
