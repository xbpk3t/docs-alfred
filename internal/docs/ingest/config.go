package wikiingest

import (
	"errors"
	"fmt"
	"time"

	"github.com/creasty/defaults"

	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

const (
	defaultWikiRoot       = "wiki"
	unclassifiedTopicPath = "none"
	inboxTopicPath        = "inbox"

	StatusSummaryWritten = "summary_written"
	StatusFailureWritten = "failure_written"
	StatusUnhandledError = "unhandled_error"
	StatusDryRunSummary  = "dry_run_summary"
	StatusDryRunFailure  = "dry_run_failure"
)

// Config holds wiki workflow configuration.
type Config struct {
	AI   AIConfig   `yaml:"ai"`
	Wiki WikiConfig `yaml:"wiki"`
}

// WikiConfig contains wiki-specific workflow settings.
type WikiConfig struct {
	WikiRoot       string          `default:"wiki"    validate:"required" yaml:"wikiRoot"`
	Driver         string          `default:"opencli"                     yaml:"driver"`
	Concurrency    int             `default:"3"       validate:"gte:1"    yaml:"concurrency"`
	PerURLTimeout  int             `default:"600"     validate:"gte:1"    yaml:"perURLTimeout"`
	MaxRetries     int             `default:"6"       validate:"gte:0"    yaml:"maxRetries"`
	MaxContentSize int             `default:"20000"                       yaml:"maxContentSize"`
	Media          wikiMediaConfig `yaml:"media"`
}

// wikiMediaConfig controls media content extraction.
type wikiMediaConfig struct {
	Enabled bool `default:"true" yaml:"enabled"`
}

// AIConfig contains AI model settings.
type AIConfig struct {
	APIKey      string  `yaml:"apiKey"`
	Model       string  `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL     string  `default:"https://api.lucc.dev/v1" validate:"required|url" yaml:"baseUrl"`
	Temperature float64 `default:"0.3"                     yaml:"temperature"`
}

// LoadConfig loads wiki config from disk, preserving defaults for omitted fields.
func LoadConfig(configPath, wikiRootOverride string) (*Config, error) {
	cfg, err := configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[Config]{
		Path:    configPath,
		Initial: defaultConfig(),
		AfterUnmarshal: func(cfg *Config) error {
			if wikiRootOverride != "" {
				cfg.Wiki.WikiRoot = wikiRootOverride
			}

			return nil
		},
		Validate: func(cfg *Config) error {
			return validator.Struct(cfg)
		},
	})
	if err != nil {
		return nil, formatConfigLoadError(err)
	}

	return &cfg, nil
}

func formatConfigLoadError(err error) error {
	var loadErr *configutil.LoadError
	if !errors.As(err, &loadErr) {
		return err
	}

	switch loadErr.Stage {
	case configutil.StageRead:
		return fmt.Errorf("read config: %w", loadErr.Err)
	case configutil.StageParse:
		return fmt.Errorf("parse config: %w", loadErr.Err)
	case configutil.StageUnmarshal:
		return fmt.Errorf("unmarshal config: %w", loadErr.Err)
	case configutil.StageValidate:
		return fmt.Errorf("validate config: %w", loadErr.Err)
	default:
		return err
	}
}

func defaultConfig() Config {
	var cfg Config
	defaults.MustSet(&cfg)

	return cfg
}

type inboxConfig struct {
	concurrency   int
	perURLTimeout time.Duration
	maxRetries    uint
}

func resolveInboxConfig(cfg *Config) inboxConfig {
	resolved := inboxConfig{
		concurrency:   cfg.Wiki.Concurrency,
		perURLTimeout: time.Duration(cfg.Wiki.PerURLTimeout) * time.Second,
		maxRetries:    uint(cfg.Wiki.MaxRetries),
	}
	if resolved.concurrency <= 0 {
		resolved.concurrency = 5
	}
	if resolved.perURLTimeout <= 0 {
		resolved.perURLTimeout = 3 * time.Minute
	}
	if resolved.maxRetries <= 0 {
		resolved.maxRetries = 3
	}

	return resolved
}
