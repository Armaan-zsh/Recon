package commands

import (
	"context"
	"fmt"
	"news-cli/internal/api"
	"news-cli/internal/config"
	"news-cli/internal/database"
	"news-cli/internal/feeds"
	"news-cli/internal/fetcher"
	"news-cli/internal/schedule"
	"news-cli/internal/sitegen"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

type daemonOptions struct {
	Interval time.Duration
	Port     int
	SiteDir  string
}

func GetDaemonCmd(embeddedFeeds []byte) *cobra.Command {
	var opts daemonOptions

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run Recon as a background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			feedData, err := feeds.LoadData(embeddedFeeds)
			if err != nil {
				return fmt.Errorf("feeds unavailable: %w", err)
			}

			db, err := database.InitDB()
			if err != nil {
				return fmt.Errorf("nexus DB unavailable: %w", err)
			}
			defer db.Close()

			// Clean up pre-2026 and Reddit entries on every startup.
			if err := db.PruneLowSignal(); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ prune failed: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "nexus pruned: removed stale and reddit entries")
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			server := api.NewServer(db)
			go func() {
				_ = server.Listen(ctx, opts.Port)
			}()

			ticker := time.NewTicker(opts.Interval)
			defer ticker.Stop()

			siteDir := expandHome(opts.SiteDir)

			cycle := 0
			next := time.Now()
			for {
				cycle++
				start := time.Now()
				res, err := fetcher.FetchAll(ctx, keywords, torProxy, db, feedData)
				if err == nil {
					_ = db.SetLastSyncTime(time.Now())
				}

				next = time.Now().Add(opts.Interval)
				if err != nil {
					fmt.Fprintf(os.Stderr, "daemon cycle %d failed: %v\n", cycle, err)
				} else {
					fmt.Fprintf(os.Stderr, "daemon cycle %d: %d articles (%d/%d feeds) in %s, next %s\n",
						cycle, len(res.Articles), res.FetchedFeeds, res.TotalFeeds, time.Since(start).Round(time.Second), next.Format(time.RFC3339))
					if len(res.Articles) > 0 {
						if err := sitegen.Generate(db, siteDir); err != nil {
							fmt.Fprintf(os.Stderr, "site rebuild failed: %v\n", err)
						} else {
							fmt.Fprintln(os.Stderr, "site rebuilt")
						}
					}
				}

				select {
				case <-ctx.Done():
					fmt.Fprintf(os.Stderr, "daemon exiting after %s\n", time.Since(start).Round(time.Second))
					return nil
				case <-ticker.C:
				}
			}
		},
	}

	cmd.PersistentFlags().DurationVar(&opts.Interval, "interval", 15*time.Minute, "Fetch interval")
	cmd.PersistentFlags().IntVar(&opts.Port, "port", 9645, "API listen port")
	cmd.PersistentFlags().StringVar(&opts.SiteDir, "site-dir", "~/.config/recon/site", "Static site output directory")

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install systemd user service for the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.InstallDaemon(opts.Interval.String(), opts.Port)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable the daemon systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.DisableDaemon()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show daemon systemd service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return schedule.StatusDaemon()
		},
	})

	return cmd
}

func expandHome(p string) string {
	if p == "" {
		return p
	}
	if p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if len(p) == 1 {
		return home
	}
	if len(p) >= 2 && p[1] == '/' {
		return filepath.Join(home, p[2:])
	}
	return p
}
