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

type AppConfig struct {
	Timezone      string   `json:"timezone" yaml:"timezone"`
	ScheduleTime  string   `json:"schedule_time" yaml:"schedule_time"`
	Categories    []string `json:"categories" yaml:"categories"`
	Keywords      []string `json:"keywords" yaml:"keywords"`
	SetupComplete bool     `json:"setup_complete" yaml:"setup_complete"`
	LastRun       string   `json:"last_run,omitempty" yaml:"last_run,omitempty"`
	TorProxy      string   `json:"tor_proxy,omitempty" yaml:"tor_proxy,omitempty"`

	Scoring       ScoringConfig       `json:"scoring,omitempty" yaml:"scoring,omitempty"`
	Feeds         FeedsConfig         `json:"feeds,omitempty" yaml:"feeds,omitempty"`
	Notifications NotificationsConfig `json:"notifications,omitempty" yaml:"notifications,omitempty"`
	Logging       LoggingConfig       `json:"logging,omitempty" yaml:"logging,omitempty"`
	Retention     RetentionConfig     `json:"retention,omitempty" yaml:"retention,omitempty"`
}

type ScoringConfig struct {
	MinScoreThreshold int            `json:"min_score_threshold" yaml:"min_score_threshold"`
	WorkerLimit       int            `json:"worker_limit" yaml:"worker_limit"`
	HighValueSources  map[string]int `json:"high_value_sources" yaml:"high_value_sources"`
	CVEBaseScore      int            `json:"cve_base_score" yaml:"cve_base_score"`
	NarrativeBonus    int            `json:"narrative_bonus" yaml:"narrative_bonus"`
	LowSignalPenalty  int            `json:"low_signal_penalty" yaml:"low_signal_penalty"`
	FluffPenalty      int            `json:"fluff_penalty" yaml:"fluff_penalty"`
	CategoryWeights   map[string]int `json:"category_weights" yaml:"category_weights"`
}

type FeedsConfig struct {
	EnableDragnet      bool                    `json:"enable_dragnet" yaml:"enable_dragnet"`
	FeedOverrides      map[string]FeedOverride `json:"feed_overrides" yaml:"feed_overrides"`
	BlacklistedDomains []string                `json:"blacklisted_domains" yaml:"blacklisted_domains"`
	WhitelistedDomains []string                `json:"whitelisted_domains" yaml:"whitelisted_domains"`
	ProxyPerFeed       map[string]string       `json:"proxy_per_feed" yaml:"proxy_per_feed"`
}

type FeedOverride struct {
	ScoreBoost   int      `json:"score_boost" yaml:"score_boost"`
	ScorePenalty int      `json:"score_penalty" yaml:"score_penalty"`
	Categories   []string `json:"categories" yaml:"categories"`
}

type NotificationsConfig struct {
	SlackWebhook      string `json:"slack_webhook,omitempty" yaml:"slack_webhook,omitempty"`
	DiscordWebhook    string `json:"discord_webhook,omitempty" yaml:"discord_webhook,omitempty"`
	EmailSMTPServer   string `json:"email_smtp_server,omitempty" yaml:"email_smtp_server,omitempty"`
	EmailSMTPPort     int    `json:"email_smtp_port,omitempty" yaml:"email_smtp_port,omitempty"`
	EmailFrom         string `json:"email_from,omitempty" yaml:"email_from,omitempty"`
	EmailTo           string `json:"email_to,omitempty" yaml:"email_to,omitempty"`
	ObsidianVault     string `json:"obsidian_vault,omitempty" yaml:"obsidian_vault,omitempty"`
	BreakingNewsScore int    `json:"breaking_news_score" yaml:"breaking_news_score"`
	EnableRSS         bool   `json:"enable_rss" yaml:"enable_rss"`
	RSSOutputPath     string `json:"rss_output_path" yaml:"rss_output_path"`
}

type LoggingConfig struct {
	Level      string `json:"level" yaml:"level"`
	Format     string `json:"format" yaml:"format"`
	OutputPath string `json:"output_path" yaml:"output_path"`
}

type RetentionConfig struct {
	ArticleRetentionDays int  `json:"article_retention_days" yaml:"article_retention_days"`
	EnableCompression    bool `json:"enable_compression" yaml:"enable_compression"`
}

type CategoryDef struct {
	Name     string   `yaml:"name"`
	ID       string   `yaml:"id"`
	Keywords []string `yaml:"keywords"`
}

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

func DefaultConfig() *AppConfig {
	return &AppConfig{
		SetupComplete: false,
		Scoring: ScoringConfig{
			MinScoreThreshold: 5,
			WorkerLimit:       500,
			HighValueSources:  make(map[string]int),
			CVEBaseScore:      15,
			NarrativeBonus:    10,
			LowSignalPenalty:  25,
			FluffPenalty:      40,
			CategoryWeights:   make(map[string]int),
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
		Retention: RetentionConfig{
			ArticleRetentionDays: 7,
		},
	}
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func LoadConfig() (*AppConfig, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	yamlPath := filepath.Join(dir, "config.yaml")
	jsonPath := filepath.Join(dir, "config.json")

	var data []byte
	var isJSON bool

	if _, errY := os.Stat(yamlPath); errY == nil {
		data, _ = os.ReadFile(yamlPath)
	} else if _, errJ := os.Stat(jsonPath); errJ == nil {
		data, _ = os.ReadFile(jsonPath)
		isJSON = true
	}

	if len(data) == 0 {
		return DefaultConfig(), nil
	}

	var cfg AppConfig
	if isJSON {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.json: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
		}
	}

	mergeConfig(&cfg, DefaultConfig())
	return &cfg, nil
}

func mergeConfig(cfg, defaults *AppConfig) {
	if cfg.Scoring.WorkerLimit == 0 {
		cfg.Scoring.WorkerLimit = defaults.Scoring.WorkerLimit
	}
	if cfg.Scoring.MinScoreThreshold == 0 {
		cfg.Scoring.MinScoreThreshold = defaults.Scoring.MinScoreThreshold
	}
}

func SaveConfig(cfg *AppConfig) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")
	return os.WriteFile(path, data, 0600)
}

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

func RecordLastRun(cfg *AppConfig) error {
	cfg.LastRun = time.Now().Format("2006-01-02")
	return SaveConfig(cfg)
}

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
