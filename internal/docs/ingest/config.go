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

// Config holds wiki workflow configuration shared by wiki subcommands.
type Config struct {
	Compact CompactConfig `yaml:"compact"`
	AI      AIConfig      `yaml:"ai"`
	Wiki    WikiConfig    `yaml:"wiki"`
}

// CompactConfig is wiki compact–only settings (delivery + brand + schedule).
// Secrets stay in env: RESEND_TOKEN, LINEAR_API_KEY.
type CompactConfig struct {
	Title string            `default:"wiki compact" yaml:"title"`
	Send  CompactSendConfig `yaml:"send"`
	// Schedule is the compact cadence in weeks: 1 = weekly, 2 = every other
	// week, … The CLI skips runs whose current week index is not a multiple
	// of schedule, so actions may trigger daily — only in-window runs send.
	Schedule int `default:"1" yaml:"schedule" validate:"gte:1"`
}

// CompactSendConfig groups dual-delivery channel parameters.
type CompactSendConfig struct {
	Resend CompactResendConfig `yaml:"resend"`
	Linear CompactLinearConfig `yaml:"linear"`
}

// CompactResendConfig is Resend delivery for compact (token only from env).
type CompactResendConfig struct {
	MailTo []string `yaml:"mailTo"`
}

// CompactLinearConfig is Linear create-issue settings for compact
// (API key only from env LINEAR_API_KEY).
//
// Defaults (overridable): teamKey=LUC, stateName=In Review, priority=2 (High),
// assignee=viewer (current API key user). Set assignee to "none" to leave
// unassigned, or a user UUID to pin someone else.
type CompactLinearConfig struct {
	TeamKey   string `default:"LUC"       yaml:"teamKey"`
	StateName string `default:"In Review" yaml:"stateName"`
	// Assignee: "viewer" | "me" | "none" | user UUID.
	Assignee string `default:"viewer"    yaml:"assignee"`
	// Priority: 0 none, 1 urgent, 2 high, 3 medium, 4 low.
	Priority int `default:"2"         yaml:"priority" validate:"gte:0|lte:4"`
}

// WikiConfig contains wiki-specific workflow settings.
type WikiConfig struct {
	WikiRoot       string          `default:"wiki"    validate:"required" yaml:"wikiRoot"`
	Driver         string          `default:"opencli"                     yaml:"driver"`
	Concurrency    int             `default:"3"       validate:"gte:1"    yaml:"concurrency"`
	PerURLTimeout  int             `default:"600"     validate:"gte:1"    yaml:"perURLTimeout"`
	MaxContentSize int             `default:"20000"                       yaml:"maxContentSize"`
	Media          wikiMediaConfig `yaml:"media"`
}

// wikiMediaConfig controls media content extraction.
type wikiMediaConfig struct {
	Enabled bool `default:"true" yaml:"enabled"`
}

// AIConfig contains AI model settings.
// Streaming is owned by pkg/ai.DefaultConfig (true by default); not a YAML knob.
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
}

func resolveInboxConfig(cfg *Config) inboxConfig {
	resolved := inboxConfig{
		concurrency:   cfg.Wiki.Concurrency,
		perURLTimeout: time.Duration(cfg.Wiki.PerURLTimeout) * time.Second,
	}
	if resolved.concurrency <= 0 {
		resolved.concurrency = 5
	}
	if resolved.perURLTimeout <= 0 {
		resolved.perURLTimeout = 3 * time.Minute
	}

	return resolved
}
