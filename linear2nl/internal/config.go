package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	env "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the top-level configuration for linear2nl.
type Config struct {
	Resend  ResendConfig  `koanf:"resend"`
	Theme   string        `koanf:"theme"`
	Morning MorningConfig `koanf:"morning"`
	AI      AIConfig      `koanf:"ai"`
	Linear  LinearConfig  `koanf:"linear"`
}

// LinearConfig holds Linear API configuration.
type LinearConfig struct {
	APIKey   string   `koanf:"apiKey"`
	TeamKeys []string `koanf:"teamKeys"`
}

// MorningConfig holds morning report configuration.
type MorningConfig struct {
	Strategy string `koanf:"strategy"`
}

// AIConfig holds AI summary configuration.
type AIConfig struct {
	Model    string        `koanf:"model"`
	Language string        `koanf:"language"`
	APIKey   string        `koanf:"apiKey"`
	BaseURL  string        `koanf:"baseURL"`
	Timeout  time.Duration `koanf:"timeout"`
}

// ResendConfig holds Resend email configuration.
type ResendConfig struct {
	Token    string   `koanf:"token"`
	FromName string   `koanf:"fromName"`
	MailTo   []string `koanf:"mailTo"`
}

// LoadConfig reads and validates configuration from the given YAML file.
// Env var overrides (applied after YAML, so they win):
//
//	LINEAR2NL_MORNING_STRATEGY  → morning.strategy
//	LINEAR2NL_AI_MODEL          → ai.model
//	LINEAR_API_KEY              → linear.apiKey
//	RESEND_TOKEN                → resend.token
//
// AI config flows to pkg/ai.DefaultConfig() (OPENAI_API_KEY etc.).
func LoadConfig(path string) (*Config, error) {
	k := koanf.New(".")

	// 1. Load YAML config file
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// 2. Env var overrides and fallbacks. Load after YAML so env wins.
	if err := loadEnvOverrides(k); err != nil {
		return nil, fmt.Errorf("load env config: %w", err)
	}

	// 3. Unmarshal into struct
	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// AI apiKey/baseURL fall through to pkg/ai.DefaultConfig()
	// which reads OPENAI_API_KEY / OPENAI_BASE_URL / LLM_MODEL.

	applyDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func loadEnvOverrides(k *koanf.Koanf) error {
	return k.Load(env.Provider(".", env.Opt{TransformFunc: func(key, value string) (string, any) {
		if value == "" {
			return "", nil
		}
		switch key {
		case "LINEAR2NL_MORNING_STRATEGY":
			return "morning.strategy", value
		case "LINEAR2NL_AI_MODEL":
			return "ai.model", value
		case "LINEAR_API_KEY":
			return "linear.apiKey", value
		case "RESEND_TOKEN":
			return "resend.token", value
		default:
			return "", nil
		}
	}}), nil)
}

func applyDefaults(cfg *Config) {
	if cfg.Theme == "" {
		cfg.Theme = "dark"
	}
	if cfg.Morning.Strategy == "" {
		cfg.Morning.Strategy = "all_assigned"
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "deepseek-v4-flash"
	}
	if cfg.AI.Language == "" {
		cfg.AI.Language = "zh"
	}
	if cfg.Resend.FromName == "" {
		cfg.Resend.FromName = "Linear Bot"
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Linear.APIKey == "" {
		return errors.New("linear API key is required (set linear.apiKey or LINEAR_API_KEY)")
	}
	if cfg.Resend.Token == "" {
		return errors.New("resend token is required (set resend.token or RESEND_TOKEN)")
	}
	if len(cfg.Resend.MailTo) == 0 {
		return errors.New("resend.mailTo is required")
	}
	if cfg.Morning.Strategy != "" &&
		cfg.Morning.Strategy != "all_assigned" &&
		cfg.Morning.Strategy != "focused" {
		return fmt.Errorf("morning.strategy must be 'all_assigned' or 'focused', got %q", cfg.Morning.Strategy)
	}
	if cfg.Theme != "dark" && cfg.Theme != "light" {
		return fmt.Errorf("theme must be 'dark' or 'light', got %q", cfg.Theme)
	}

	return nil
}
