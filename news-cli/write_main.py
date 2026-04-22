#!/usr/bin/env python3
import os

content = r'''package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"news-cli/cmd/commands"
	"news-cli/internal/config"
	"news-cli/internal/database"
	"news-cli/internal/feeds"
	"news-cli/internal/fetcher"
	"news-cli/internal/models"
	"news-cli/internal/notifier"
	"news-cli/internal/renderer"
	"news-cli/internal/schedule"
	"news-cli/internal/scorer"
	"news-cli/internal/tui"
	"os"
	"time"

	"github.com/spf13/cobra"
)

//go:embed links.json
var linksData []byte

func main() {
	rootCmd := &cobra.Command{
		Use:   "recon",
		Short: "Recon - High-Signal Intelligence Nexus",
		RunE:  runDefault,
	}

	rootCmd.Flags().Bool("json", false, "Output results as JSON to stdout")
	rootCmd.Flags().Bool("browser", false, "Open results in browser sidebar view")

	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage the daily intelligence timer",
	}

	scheduleCmd.AddCommand(&cobra.Command{
		Use:   "install [time]",
		Short: "Install systemd timer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.Install(args[0])
		},
	})

	scheduleCmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable the daily timer",
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.Disable()
		},
	})

	scheduleCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show timer status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.Status()
		},
	})

	intelCmd := &cobra.Command{
		Use:   "intel",
		Short: "Manage threat intelligence sources",
	}

	intelCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Download the latest CISA KEV and EPSS data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return scorer.UpdateIntel()
		},
	})

	rootCmd.AddCommand(scheduleCmd)
	rootCmd.AddCommand(intelCmd)
	rootCmd.AddCommand(commands.GetFeedsCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDefault(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
	}

	var keywords []string
	var torProxy string
	if cfg != nil {
		keywords = config.KeywordsForCategories(cfg.Categories)
		keywords = append(keywords, cfg.Keywords...)
		torProxy = cfg.TorProxy
	}
	fmt.Fprintf(os.Stderr, "Loaded %d keywords.\n", len(keywords))

	feedData, err := feeds.LoadData(linksData)
	if err != nil {
		return fmt.Errorf("feeds unavailable: %w", err)
	}

	db, err := database.InitDB()
	if err != nil {
		return fmt.Errorf("nexus DB unavailable: %w", err)
	}
	defer db.Close()

	articles, err := loadArticles(context.Background(), db, keywords, torProxy, feedData)
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(articles)
	}

	useBrowser, _ := cmd.Flags().GetBool("browser")
	if useBrowser {
		htmlContent, err := renderer.RenderHTML(articles)
		if err != nil {
			return err
		}
		renderer.ServeAndOpen(htmlContent)
		return nil
	}

	return tui.RunTUI(articles, keywords, torProxy, feedData)
}

func loadArticles(ctx context.Context, db *database.IntelligenceDB, keywords []string, torProxy string, feedData []byte) ([]models.Article, error) {
	articles, err := db.GetRecentArticles(200)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent articles: %w", err)
	}
	currentHashes := make(map[string]bool, len(articles))
	for _, article := range articles {
		currentHashes[article.Hash()] = true
	}

	lastSync := db.GetLastSyncTime()
	needsSync := len(articles) == 0 || lastSync.IsZero() || time.Since(lastSync) > 4*time.Hour
	if !needsSync {
		return articles, nil
	}

	if len(articles) == 0 {
		fmt.Fprintln(os.Stderr, "No recent articles in cache. Fetching fresh intelligence...")
	} else if lastSync.IsZero() {
		fmt.Fprintln(os.Stderr, "Cache has data but no sync marker. Refreshing intelligence...")
	} else {
		fmt.Fprintf(os.Stderr, "Last sync was %s ago. Refreshing intelligence...\n", time.Since(lastSync).Round(time.Minute))
	}

	res, err := fetcher.FetchAll(ctx, keywords, torProxy, db, feedData)
	if err != nil {
		if len(articles) > 0 {
			fmt.Fprintf(os.Stderr, "Sync failed, showing cached articles: %v\n", err)
			return articles, nil
		}
		return nil, err
	}

	_ = db.SetLastSyncTime(time.Now())

	refreshed, err := db.GetRecentArticles(200)
	if err == nil && len(refreshed) > 0 {
		maybeNotify(currentHashes, refreshed)
		return refreshed, nil
	}
	if len(res.Articles) > 0 {
		maybeNotify(currentHashes, res.Articles)
		return res.Articles, nil
	}
	return articles, nil
}

func maybeNotify(currentHashes map[string]bool, articles []models.Article) {
	if len(articles) == 0 {
		return
	}

	newCount := 0
	var lead models.Article
	for _, article := range articles {
		if !currentHashes[article.Hash()] {
			if newCount == 0 {
				lead = article
			}
			newCount++
		}
	}
	if newCount > 0 {
		notifier.NotifyNewArticles(newCount, lead)
	}
}
'''

target = os.path.expanduser("~/Documents/recon/news-cli/cmd/recon/main.go")
with open(target, "w") as f:
    f.write(content.lstrip("\n"))
print(f"Written {os.path.getsize(target)} bytes to main.go")
