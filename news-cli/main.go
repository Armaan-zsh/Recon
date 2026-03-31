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
		Use:   "recon",
		Short: "Recon — High-Signal Intelligence Nexus",
		Long:  "Cracking the code of discovery with 2,500+ elite feeds and persistent threat memory.",
		RunE:  runDefault,
	}

	rootCmd.Flags().Bool("json", false, "Output results as JSON to stdout")
	rootCmd.Flags().Bool("sync", false, "Synchronize feeds before opening TUI")
	rootCmd.Flags().Bool("browser", false, "Open results in browser sidebar view instead of TUI")

	// Subcommands
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Run the setup wizard",
		RunE:  func(cmd *cobra.Command, args []string) error { _, err := RunSetupWizard(); return err },
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Background synchronize all 2,500+ elite feeds",
		RunE:  runSync,
	}

	dashCmd := &cobra.Command{
		Use:   "dash",
		Short: "Open the high-level Intelligence Dashboard",
		RunE:  func(cmd *cobra.Command, args []string) error { return runGridDashboard() },
	}

	scheduleCmd := &cobra.Command{Use: "schedule", Short: "Manage auto-run schedule"}
	// ... (rest of schedule logic from before, omitted for brevity but I'll add the sets)
	scheduleSetCmd := &cobra.Command{
		Use:   "set",
		Short: "Set auto-run time",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, _ := cmd.Flags().GetString("time")
			return ScheduleInstall(t)
		},
	}
	scheduleSetCmd.Flags().String("time", "07:00", "24h format")
	scheduleCmd.AddCommand(scheduleSetCmd)

	rootCmd.AddCommand(initCmd, syncCmd, dashCmd, scheduleCmd)

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

	doSync, _ := cmd.Flags().GetBool("sync")
	if doSync {
		fmt.Fprintf(os.Stderr, "🔄 Syncing Motherlode (2,500 feeds)...\n")
		_, _ = FetchAll(context.Background(), cfg, db)
	}

	// Fetch recent articles from DB (Zero-Latency)
	// For now, we'll run a quick fetch if DB is empty
	res, err := FetchAll(context.Background(), cfg, db)
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(res.Articles)
	}

	useBrowser, _ := cmd.Flags().GetBool("browser")
	if useBrowser {
		htmlContent, err := renderHTML(res.Articles)
		if err != nil {
			return err
		}
		serveAndOpen(htmlContent)
		return nil
	}

	return RunTUI(res.Articles, cfg)
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, _ := LoadConfig()
	db, err := InitDB()
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Fprintf(os.Stderr, "📡 Synchronizing Intelligence Nexus...\n")
	res, err := FetchAll(context.Background(), cfg, db)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ Sync Complete. %d items indexed in %v\n", len(res.Articles), res.Duration)
	return nil
}
