package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"news-cli/internal/config"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"
)

type torCheckResp struct {
	IP    string `json:"IP"`
	IsTor bool   `json:"IsTor"`
}

func GetDoctorCmd() *cobra.Command {
	var proxyOverride string

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run environment and connectivity diagnostics",
	}

	torCmd := &cobra.Command{
		Use:   "tor",
		Short: "Check Tor proxy routing health",
		RunE: func(cmd *cobra.Command, args []string) error {
			proxyURL := strings.TrimSpace(proxyOverride)
			if proxyURL == "" {
				cfg, err := config.LoadConfig()
				if err == nil && cfg != nil {
					proxyURL = strings.TrimSpace(cfg.TorProxy)
				}
			}
			if proxyURL == "" {
				return fmt.Errorf("tor proxy not configured; set tor_proxy in config or use --proxy")
			}

			fmt.Printf("Checking Tor proxy: %s\n", proxyURL)
			resp, err := checkTorProxy(proxyURL)
			if err != nil {
				return fmt.Errorf("tor check failed: %w", err)
			}
			if !resp.IsTor {
				return fmt.Errorf("proxy reachable but not routing through Tor (ip=%s)", resp.IP)
			}
			fmt.Printf("✅ Tor routing healthy. Exit IP: %s\n", resp.IP)
			return nil
		},
	}
	torCmd.Flags().StringVar(&proxyOverride, "proxy", "", "Tor SOCKS proxy URL (e.g. socks5://127.0.0.1:9050)")

	doctorCmd.AddCommand(torCmd)
	return doctorCmd
}

func checkTorProxy(proxyRaw string) (*torCheckResp, error) {
	proxyURL, err := url.Parse(proxyRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to build proxy dialer: %w", err)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
	}

	req, _ := http.NewRequest("GET", "https://check.torproject.org/api/ip", nil)
	req.Header.Set("User-Agent", "Recon/2.0 doctor")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("tor check status %d", res.StatusCode)
	}

	var out torCheckResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode tor check response: %w", err)
	}
	return &out, nil
}
