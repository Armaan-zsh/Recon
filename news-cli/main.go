package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

//go:embed links.json
var linksData []byte

func LoadFeeds() ([]FeedSource, error) {
	var payload struct {
		Links []FeedSource `json:"links"`
	}
	if err := json.Unmarshal(linksData, &payload); err != nil {
		return nil, err
	}
	return payload.Links, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:	"recon",
		Short:	"Recon — High-Signal Intelligence Nexus",
		Long:	"Cracking the code of discovery with 2,500+ elite feeds and persistent threat memory.",
		RunE:	runDefault,
	}

	rootCmd.Flags().Bool("json", false, "Output results as JSON to stdout")
	rootCmd.Flags().Bool("browser", false, "Open results in browser sidebar view instead of TUI")

	initCmd := &cobra.Command{
		Use:	"init",
		Short:	"Run the setup wizard",
		RunE:	func(cmd *cobra.Command, args []string) error { _, err := RunSetupWizard(); return err },
	}

	rootCmd.AddCommand(initCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDefault(cmd *cobra.Command, args []string) error {
	cfg, _ := LoadConfig()
	if cfg == nil {
		cfg, _ = RunSetupWizard()
	}

	db, err := InitDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Nexus DB unavailable: %v\n", err)
	} else {
		defer db.Close()
	}

	// Fetch recent articles from DB (Zero-Latency)
	var articles []Article
	if db != nil {
		articles, _ = db.GetRecentArticles(200)
	}

	if len(articles) == 0 {
		fmt.Fprintf(os.Stderr, "⚠ DB empty. Performing initial fetch... this may take a minute.\n")
		res, err := FetchAll(context.Background(), cfg, db)
		if err != nil {
			return err
		}
		articles = res.Articles
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(articles)
	}

	useBrowser, _ := cmd.Flags().GetBool("browser")
	if useBrowser {
		htmlContent, err := renderHTML(articles)
		if err != nil {
			return err
		}
		serveAndOpen(htmlContent)
		return nil
	}

	return RunTUI(articles, cfg)
}
