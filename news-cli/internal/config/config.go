package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const appName = "recon"

// AppConfig holds all user preferences from the setup wizard.
type AppConfig struct {
	Timezone      string   `json:"timezone" yaml:"timezone"`
	ScheduleTime  string   `json:"schedule_time" yaml:"schedule_time"`
	Categories    []string `json:"categories" yaml:"categories"`
	Keywords      []string `json:"keywords" yaml:"keywords"`
	SetupComplete bool     `json:"setup_complete" yaml:"setup_complete"`
	LastRun       string   `json:"last_run,omitempty" yaml:"last_run,omitempty"`
	TorProxy      string   `json:"tor_proxy,omitempty" yaml:"tor_proxy,omitempty"` // SOCKS5 proxy for .onion searches
	
	// New configuration options
	Scoring       ScoringConfig       `json:"scoring,omitempty" yaml:"scoring,omitempty"`
	Feeds         FeedsConfig         `json:"feeds,omitempty" yaml:"feeds,omitempty"`
	Notifications NotificationsConfig `json:"notifications,omitempty" yaml:"notifications,omitempty"`
	Logging       LoggingConfig       `json:"logging,omitempty" yaml:"logging,omitempty"`
	Retention     RetentionConfig     `json:"retention,omitempty" yaml:"retention,omitempty"`
}

// ScoringConfig holds configurable score thresholds and weights.
type ScoringConfig struct {
	MinScoreThreshold    int                `json:"min_score_threshold" yaml:"min_score_threshold"`
	WorkerLimit          int                `json:"worker_limit" yaml:"worker_limit"`
	HighValueSources     map[string]int     `json:"high_value_sources" yaml:"high_value_sources"`
	CVEBaseScore         int                `json:"cve_base_score" yaml:"cve_base_score"`
	NarrativeBonus       int                `json:"narrative_bonus" yaml:"narrative_bonus"`
	LowSignalPenalty     int                `json:"low_signal_penalty" yaml:"low_signal_penalty"`
	FluffPenalty         int                `json:"fluff_penalty" yaml:"fluff_penalty"`
	CategoryWeights      map[string]int     `json:"category_weights" yaml:"category_weights"`
}

// FeedsConfig holds feed-specific overrides and settings.
type FeedsConfig struct {
	EnableDragnet      bool              `json:"enable_dragnet" yaml:"enable_dragnet"`
	FeedOverrides      map[string]FeedOverride `json:"feed_overrides" yaml:"feed_overrides"`
	BlacklistedDomains []string          `json:"blacklisted_domains" yaml:"blacklisted_domains"`
	WhitelistedDomains []string          `json:"whitelisted_domains" yaml:"whitelisted_domains"`
	ProxyPerFeed       map[string]string `json:"proxy_per_feed" yaml:"proxy_per_feed"`
}

// FeedOverride allows per-feed scoring adjustments.
type FeedOverride struct {
	ScoreBoost   int      `json:"score_boost" yaml:"score_boost"`
	ScorePenalty int      `json:"score_penalty" yaml:"score_penalty"`
	Categories   []string `json:"categories" yaml:"categories"`
}

// NotificationsConfig holds notification channel settings.
type NotificationsConfig struct {
	SlackWebhook     string `json:"slack_webhook,omitempty" yaml:"slack_webhook,omitempty"`
	DiscordWebhook   string `json:"discord_webhook,omitempty" yaml:"discord_webhook,omitempty"`
	EmailSMTPServer  string `json:"email_smtp_server,omitempty" yaml:"email_smtp_server,omitempty"`
	EmailSMTPPort    int    `json:"email_smtp_port,omitempty" yaml:"email_smtp_port,omitempty"`
	EmailFrom        string `json:"email_from,omitempty" yaml:"email_from,omitempty"`
	EmailTo          string `json:"email_to,omitempty" yaml:"email_to,omitempty"`
	ObsidianVault    string `json:"obsidian_vault,omitempty" yaml:"obsidian_vault,omitempty"`
	BreakingNewsScore int   `json:"breaking_news_score" yaml:"breaking_news_score"`
	EnableRSS        bool   `json:"enable_rss" yaml:"enable_rss"`
	RSSOutputPath    string `json:"rss_output_path" yaml:"rss_output_path"`
}

// LoggingConfig controls logging behavior.
type LoggingConfig struct {
	Level      string `json:"level" yaml:"level"` // debug, info, warn, error
	Format     string `json:"format" yaml:"format"` // json, text
	OutputPath string `json:"output_path" yaml:"output_path"`
}

// RetentionConfig controls data retention policies.
type RetentionConfig struct {
	ArticleRetentionDays int  `json:"article_retention_days" yaml:"article_retention_days"`
	EnableCompression    bool `json:"enable_compression" yaml:"enable_compression"`
}

// CategoryDef maps a human-readable category to its scoring keywords.
type CategoryDef struct {
	Name     string   `yaml:"name"`
	ID       string   `yaml:"id"`
	Keywords []string `yaml:"keywords"`
}

// AllCategories is the master list of categories the user can choose from.
var AllCategories = []CategoryDef{
	{Name: "Vulnerabilities & CVEs", ID: "vulnerabilities", Keywords: []string{"vulnerability", "cve", "patch", "advisory", "exploit", "rce", "remote code execution", "buffer overflow", "privilege escalation"}},
	{Name: "Malware & Threat Intel", ID: "malware", Keywords: []string{"malware", "ransomware", "trojan", "apt", "threat actor", "c2", "command and control", "botnet", "backdoor", "infostealer"}},
	{Name: "Zero-Days & Exploits", ID: "zero-days", Keywords: []string{"zero-day", "0day", "in-the-wild", "proof-of-concept", "poc", "exploit chain", "weaponized"}},
	{Name: "Data Breaches & Leaks", ID: "breaches", Keywords: []string{"breach", "leak", "exposed data", "credential", "dump", "stolen data", "data exposure", "compromised"}},
	{Name: "Cloud & Infra Security", ID: "cloud", Keywords: []string{"cloud security", "aws security", "azure security", "gcp", "misconfiguration", "s3 bucket", "iam", "container escape", "kubernetes security", "docker security"}},
	{Name: "Cryptography", ID: "cryptography", Keywords: []string{"cryptography", "encryption", "tls", "certificate", "pki", "post-quantum", "side-channel", "key exchange"}},
	{Name: "AI & Machine Learning", ID: "ai", Keywords: []string{"artificial intelligence", "machine learning", "llm", "neural", "gpt", "deep learning", "generative ai", "prompt injection"}},
	{Name: "Privacy & Surveillance", ID: "privacy", Keywords: []string{"privacy", "surveillance", "tracking", "gdpr", "anonymity", "tor", "vpn", "data protection", "spyware"}},
	{Name: "Nation-State & APT", ID: "nation-state", Keywords: []string{"apt", "nation state", "state sponsored", "cyber warfare", "government hacker", "fancy bear", "cozy bear", "lazarus", "turla", "sandworm"}},
	{Name: "Dark Web", ID: "dark-web", Keywords: []string{"dark web", "onion", "breach forum", "exploit market", "stolen credentials", "darknet", "tor hidden service"}},
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

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		SetupComplete: false,
		Scoring: ScoringConfig{
			MinScoreThreshold:    5,
			WorkerLimit:          500,
			HighValueSources:     make(map[string]int),
			CVEBaseScore:         15,
			NarrativeBonus:       10,
			LowSignalPenalty:     25,
			FluffPenalty:         40,
			CategoryWeights:      make(map[string]int),
		},
		Feeds: FeedsConfig{
			EnableDragnet:      false,
			FeedOverrides:      make(map[string]FeedOverride),
			BlacklistedDomains: []string{},
			WhitelistedDomains: []string{},
			ProxyPerFeed:       make(map[string]string),
		},
		Notifications: NotificationsConfig{
			BreakingNewsScore: 80,
			EnableRSS:         false,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Retention: RetentionConfig{
			ArticleRetentionDays: 7,
			EnableCompression:    false,
		},
	}
}

// configDir returns the path to the application's config directory.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine config directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}

// configFilePath returns the full path to config.yaml.
func configFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return func() string { if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil { return filepath.Join(dir, "config.yaml") }; return filepath.Join(dir, "config.json") }(), nil
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
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Try JSON for backward compatibility
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse config (YAML/JSON): %w", err)
		}
	}
	
	// Merge with defaults for any missing fields
	defaults := DefaultConfig()
	mergeConfig(&cfg, defaults)
	
	return &cfg, nil
}

// mergeConfig merges default values into the loaded config for missing fields.
func mergeConfig(cfg, defaults *AppConfig) {
	if cfg.Scoring.WorkerLimit == 0 {
		cfg.Scoring.WorkerLimit = defaults.Scoring.WorkerLimit
	}
	if cfg.Scoring.MinScoreThreshold == 0 {
		cfg.Scoring.MinScoreThreshold = defaults.Scoring.MinScoreThreshold
	}
	if cfg.Retention.ArticleRetentionDays == 0 {
		cfg.Retention.ArticleRetentionDays = defaults.Retention.ArticleRetentionDays
	}
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

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := func() string { if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil { return filepath.Join(dir, "config.yaml") }; return filepath.Join(dir, "config.json") }()
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
