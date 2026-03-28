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
	"time"

	"github.com/spf13/cobra"
)

//go:embed links.json
var linksData []byte

type LinksConfig struct {
	Links []struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"links"`
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
	// Dark subcommand
	darkCmd := &cobra.Command{
		Use:   "dark [query]",
		Short: "Search the dark web for intelligence (requires Tor)",
		RunE:  runDarkSearch,
	}

	rootCmd.AddCommand(initCmd, scheduleCmd, darkCmd)

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
	for _, blog := range feedsConfig.Links {
		sources = append(sources, FeedSource{Name: blog.Name, URL: blog.Url})
	}

	// Output mode
	jsonMode, _ := cmd.Flags().GetBool("json")
	browserMode, _ := cmd.Flags().GetBool("browser")

	if jsonMode || browserMode {
		stopSpinner := make(chan bool)
		go func() {
			chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			i := 0
			for {
				select {
				case <-stopSpinner:
					fmt.Fprintf(os.Stderr, "\r\033[K")
					return
				default:
					fmt.Fprintf(os.Stderr, "\r\033[38;2;249;115;22m%s\033[0m Gathering Cyber Intelligence (%d feeds)...", chars[i%len(chars)], len(sources))
					i++
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		result := FetchFeeds(context.Background(), sources, keywords, strictFilter, cfg)
		stopSpinner <- true
		
		sort.Slice(result.Articles, func(i, j int) bool {
			if result.Articles[i].Score == result.Articles[j].Score {
				return result.Articles[i].Published.After(result.Articles[j].Published)
			}
			return result.Articles[i].Score > result.Articles[j].Score
		})
		
		topN := 50
		if len(result.Articles) < topN {
			topN = len(result.Articles)
		}
		topArticles := result.Articles[:topN]
		
		// We don't need log.Printf here because the output handles it cleanly or is JSON.

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
	}

	// Default: Async TUI mode
	return runTUI(sources, keywords, strictFilter, cfg)
}

func runDarkSearch(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a search query")
	}
	query := strings.Join(args, " ")

	cfg, _ := LoadConfig()
	if cfg == nil || cfg.TorProxy == "" {
		fmt.Println("⚠ Tor Proxy not configured in config.json. Please set 'tor_proxy' (e.g. socks5h://127.0.0.1:9050)")
		return nil
	}

	fmt.Printf("🕵  Searching Dark Web for: %s...\n", query)

	// In a real implementation, we'd scrape Ahmia/OnionLand here.
	// For now, we'll use our fetcher to hit known Onion research feeds if we have any,
	// or provide a placeholder for the integration.
	
	// Example Onion Engine Search (Ahmia)
	ahmiaURL := fmt.Sprintf("http://juhanurmihxlp77nkq76byazcldy2hlmovfu2epvl5ankdibsot4csyd.onion/search/?q=%s", query)
	
	log.Printf("Connecting to Ahmia via %s...", cfg.TorProxy)
	
	sources := []FeedSource{
		{Name: "Ahmia Search", URL: ahmiaURL},
	}
	
	// Since Ahmia doesn't return RSS easily, we'll eventually need a dedicated scraper.
	// But for this version, we'll try to fetch the page content.
	
	result := FetchFeeds(context.Background(), sources, []string{query}, false, cfg)
	
	if len(result.Articles) == 0 {
		fmt.Println("No results found or Tor proxy unreachable.")
		return nil
	}

	return runTUI(sources, []string{query}, false, cfg)
}
