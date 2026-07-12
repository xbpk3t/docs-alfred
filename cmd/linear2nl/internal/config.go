package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/creasty/defaults"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Config is the top-level configuration for linear2nl.
type Config struct {
	GitHub  GitHubConfig  `koanf:"github"`
	Theme   string        `default:"dark"  koanf:"theme" validate:"in:dark,light"`
	Morning MorningConfig `koanf:"morning"`
	AI      AIConfig      `koanf:"ai"`
	Resend  ResendConfig  `koanf:"resend"`
	Linear  LinearConfig  `koanf:"linear"`
}

// GitHubConfig holds GitHub API configuration for the review command.
type GitHubConfig struct {
	Owner string `koanf:"owner"`
	Repo  string `koanf:"repo"`
}

// LinearConfig holds Linear API configuration.
type LinearConfig struct {
	APIKey   string   `koanf:"apiKey"   validate:"required"`
	TeamKeys []string `koanf:"teamKeys"`
}

// MorningConfig holds morning report configuration.
type MorningConfig struct {
	Strategy string `default:"all_assigned" koanf:"strategy" validate:"in:all_assigned,focused"`
}

// AIConfig holds AI summary configuration.
// Streaming is not exposed here — pkg/ai.DefaultConfig enables it by default
// so all linear2nl AI calls bypass Cloudflare 524 upstream timeouts.
type AIConfig struct {
	Model    string        `default:"deepseek-v4-flash" koanf:"model"`
	Language string        `default:"zh"                koanf:"language"`
	APIKey   string        `koanf:"apiKey"`
	BaseURL  string        `koanf:"baseURL"`
	Timeout  time.Duration `koanf:"timeout"`
}

// ResendConfig holds Resend email configuration.
type ResendConfig struct {
	Token    string   `koanf:"token"        validate:"required"`
	FromName string   `default:"Linear Bot" koanf:"fromName"`
	MailTo   []string `koanf:"mailTo"       validate:"required|min_len:1"`
}

// LoadConfig reads and validates configuration from the given YAML file.
// Env var overrides (applied after YAML, so they win):
//
//	LINEAR2NL_MORNING_STRATEGY  → morning.strategy
//	LINEAR2NL_AI_MODEL          → ai.model
//	LINEAR_API_KEY              → linear.apiKey
//	RESEND_TOKEN                → resend.token
//	GITHUB_OWNER                → github.owner
//	GITHUB_REPO                 → github.repo
//
// AI config flows to pkg/ai.DefaultConfig() (OPENAI_API_KEY etc.).
func LoadConfig(path string) (*Config, error) {
	cfg, err := configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[Config]{
		Path: path,
		Tag:  "koanf",
		EnvOverrides: []configutil.EnvOverride{
			{Name: "LINEAR2NL_MORNING_STRATEGY", Path: "morning.strategy"},
			{Name: "LINEAR2NL_AI_MODEL", Path: "ai.model"},
			{Name: "LINEAR_API_KEY", Path: "linear.apiKey"},
			{Name: "RESEND_TOKEN", Path: "resend.token"},
			{Name: "GITHUB_OWNER", Path: "github.owner"},
			{Name: "GITHUB_REPO", Path: "github.repo"},
		},
		AfterUnmarshal: func(cfg *Config) error {
			applyDefaults(cfg)

			return nil
		},
		Validate: validateConfig,
	})
	if err != nil {
		return nil, formatConfigLoadError(err)
	}

	// AI apiKey/baseURL fall through to pkg/ai.DefaultConfig()
	// which reads OPENAI_API_KEY / OPENAI_BASE_URL / LLM_MODEL.

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
		return loadErr.Err
	default:
		return err
	}
}

func applyDefaults(cfg *Config) {
	defaults.MustSet(cfg)
}

func validateConfig(cfg *Config) error {
	if err := validator.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	return nil
}
