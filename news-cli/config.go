package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const appName = "recon"

// AppConfig holds all user preferences from the setup wizard.
type AppConfig struct {
	Timezone      string   `json:"timezone"`
	ScheduleTime  string   `json:"schedule_time"`
	Categories    []string `json:"categories"`
	Keywords      []string `json:"keywords"`
	SetupComplete bool     `json:"setup_complete"`
	LastRun       string   `json:"last_run,omitempty"`
}

// CategoryDef maps a human-readable category to its scoring keywords.
type CategoryDef struct {
	Name     string
	ID       string
	Keywords []string
}

// AllCategories is the master list of categories the user can choose from.
var AllCategories = []CategoryDef{
	{Name: "Vulnerabilities & CVEs", ID: "vulnerabilities", Keywords: []string{"vulnerability", "cve", "patch", "advisory", "exploit", "rce", "remote code execution", "buffer overflow", "privilege escalation"}},
	{Name: "Malware & Threat Intel", ID: "malware", Keywords: []string{"malware", "ransomware", "trojan", "apt", "threat actor", "c2", "command and control", "botnet", "backdoor", "infostealer"}},
	{Name: "Zero-Days & Exploits", ID: "zero-days", Keywords: []string{"zero-day", "0day", "in-the-wild", "proof-of-concept", "poc", "exploit chain", "weaponized"}},
	{Name: "Data Breaches & Leaks", ID: "breaches", Keywords: []string{"breach", "leak", "exposed data", "credential", "dump", "stolen data", "data exposure", "compromised"}},
	{Name: "Cloud Security", ID: "cloud", Keywords: []string{"cloud security", "aws security", "azure security", "gcp", "misconfiguration", "s3 bucket", "iam", "container escape"}},
	{Name: "Cryptography", ID: "cryptography", Keywords: []string{"cryptography", "encryption", "tls", "certificate", "pki", "post-quantum", "side-channel", "key exchange"}},
	{Name: "AI & Machine Learning", ID: "ai", Keywords: []string{"artificial intelligence", "machine learning", "llm", "neural", "gpt", "model", "deep learning", "generative ai", "prompt injection"}},
	{Name: "Web Development", ID: "webdev", Keywords: []string{"javascript", "react", "node.js", "frontend", "css", "api", "typescript", "web framework", "next.js"}},
	{Name: "DevOps & Infrastructure", ID: "devops", Keywords: []string{"kubernetes", "docker", "ci/cd", "terraform", "infrastructure", "sre", "observability", "helm", "gitops"}},
	{Name: "Privacy & Surveillance", ID: "privacy", Keywords: []string{"privacy", "surveillance", "tracking", "gdpr", "anonymity", "tor", "vpn", "data protection", "spyware"}},
}

// Timezones provides a curated list of common timezones for the setup wizard.
var Timezones = []struct {
	Label string
	Value string
}{
	{"Asia/Kolkata (IST, UTC+5:30)", "Asia/Kolkata"},
	{"America/New_York (EST, UTC-5)", "America/New_York"},
	{"America/Chicago (CST, UTC-6)", "America/Chicago"},
	{"America/Denver (MST, UTC-7)", "America/Denver"},
	{"America/Los_Angeles (PST, UTC-8)", "America/Los_Angeles"},
	{"Europe/London (GMT, UTC+0)", "Europe/London"},
	{"Europe/Berlin (CET, UTC+1)", "Europe/Berlin"},
	{"Europe/Moscow (MSK, UTC+3)", "Europe/Moscow"},
	{"Asia/Dubai (GST, UTC+4)", "Asia/Dubai"},
	{"Asia/Shanghai (CST, UTC+8)", "Asia/Shanghai"},
	{"Asia/Tokyo (JST, UTC+9)", "Asia/Tokyo"},
	{"Australia/Sydney (AEST, UTC+10)", "Australia/Sydney"},
	{"Pacific/Auckland (NZST, UTC+12)", "Pacific/Auckland"},
	{"UTC (Coordinated Universal Time)", "UTC"},
}

// configDir returns the path to the application's config directory.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine config directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}

// configFilePath returns the full path to config.json.
func configFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads the config from disk. Returns nil if not found.
func LoadConfig() (*AppConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config yet, first run
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the config to disk atomically.
func SaveConfig(cfg *AppConfig) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, data, 0600)
}

// KeywordsForCategories resolves category IDs into their keyword lists.
func KeywordsForCategories(categoryIDs []string) []string {
	seen := make(map[string]bool)
	var keywords []string
	for _, id := range categoryIDs {
		for _, cat := range AllCategories {
			if cat.ID == id {
				for _, kw := range cat.Keywords {
					if !seen[kw] {
						seen[kw] = true
						keywords = append(keywords, kw)
					}
				}
			}
		}
	}
	return keywords
}

// RecordLastRun writes today's date into the config.
func RecordLastRun(cfg *AppConfig) error {
	cfg.LastRun = time.Now().Format("2006-01-02")
	return SaveConfig(cfg)
}

// MissedRun checks if the last run was more than 24 hours ago.
func MissedRun(cfg *AppConfig) bool {
	if cfg.LastRun == "" {
		return true
	}
	last, err := time.Parse("2006-01-02", cfg.LastRun)
	if err != nil {
		return true
	}
	return time.Since(last) > 24*time.Hour
}
