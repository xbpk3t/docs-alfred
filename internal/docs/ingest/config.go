package wikiingest

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/creasty/defaults"

	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
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
	StatusSkipped        = "skipped"
)

// Config holds wiki workflow configuration shared by wiki subcommands.
type Config struct {
	R2        R2Config        `yaml:"r2,omitempty"`
	AI        AIConfig        `yaml:"ai"`
	Compact   CompactConfig   `yaml:"compact"`
	Wiki      WikiConfig      `yaml:"wiki"`
	TrnsIndex TrnsIndexConfig `yaml:"trnsIndex,omitempty"`
}

// R2Config points the digest at the R2 bucket holding media artifacts:
// transcript pages uploaded by rss2nl and transcripts uploaded by the digest.
// Credentials come from the environment (R2_AK/R2_SK with CF_R2_AK/CF_R2_SK
// fallback), never from YAML.
type R2Config struct {
	Bucket        string `yaml:"bucket,omitempty"`
	PublicBaseURL string `yaml:"publicBaseUrl,omitempty"`
	Endpoint      string `yaml:"endpoint,omitempty"`
	KeyPrefix     string `yaml:"keyPrefix,omitempty"`
}

// TrnsIndexConfig enables consuming the rss2nl trns index (index-<date>.json
// objects in the R2 bucket) so podcast episodes with uploaded transcripts are
// digested without manual inbox edits.
type TrnsIndexConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

// LitterConfig resolves the R2 uploader config, reading credentials from the
// environment. Returns nil when no bucket is configured (R2 features off).
func (c R2Config) LitterConfig() (*litter.R2Config, error) {
	if c.Bucket == "" {
		return nil, nil
	}
	if c.PublicBaseURL == "" {
		return nil, errors.New("r2: publicBaseUrl is required when bucket is set")
	}
	endpoint := resolveR2Endpoint(c)
	ak := firstEnv("R2_AK", "CF_R2_AK")
	sk := firstEnv("R2_SK", "R2_KEY", "CF_R2_SK")
	if endpoint == "" || ak == "" || sk == "" {
		return nil, errors.New("r2: missing credentials (need R2_AK with R2_SK/R2_KEY/CF_R2_SK, and an endpoint)")
	}

	return &litter.R2Config{
		Bucket:          c.Bucket,
		PublicBaseURL:   c.PublicBaseURL,
		Endpoint:        endpoint,
		KeyPrefix:       c.KeyPrefix,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
	}, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}

	return ""
}

// resolveR2Endpoint takes the endpoint from config, else R2_ENDPOINT, else the
// Cloudflare account id (CF_ACCOUNT), so the account id stays out of YAML.
func resolveR2Endpoint(c R2Config) string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	if endpoint := os.Getenv("R2_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	if account := os.Getenv("CF_ACCOUNT"); account != "" {
		return "https://" + account + ".r2.cloudflarestorage.com"
	}

	return ""
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
	// TopN is how many top-ranked 清零 candidates are surfaced per run.
	TopN int `default:"10" yaml:"topN" validate:"gte:1"`
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
	GHTopicsURL    string          `default:"https://docs.lucc.dev/gh.yml" yaml:"ghTopicsURL"`
	Concurrency    int             `default:"3"       validate:"gte:1"    yaml:"concurrency"`
	PerURLTimeout  int             `default:"600"     validate:"gte:1"    yaml:"perURLTimeout"`
	MaxContentSize int             `default:"20000"                       yaml:"maxContentSize"`
	Media          wikiMediaConfig `yaml:"media"`
	// IsAggr switches digest output layout. false (default): entries append to
	// each topic's <topic>/summary.md, grouped by date. true: one
	// <date>-summary.md per day at the wiki root, grouped by topic path.
	IsAggr bool `default:"false" yaml:"isAggr"`
}

// wikiMediaConfig controls media content extraction.
type wikiMediaConfig struct {
	Enabled bool `default:"true" yaml:"enabled"`
}

// AIConfig contains AI model settings.
// Streaming is owned by pkg/ai (ignored on the MAF path); not a YAML knob.
type AIConfig struct {
	APIKey  string `yaml:"apiKey"`
	Model   string `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL string `default:"https://api.lucc.dev/v1" validate:"required|url" yaml:"baseUrl"`
	// Effort is the reasoning-effort tier, a peer of model. Empty falls to the
	// shared pkg/ai default (LLM_EFFORT env, else "max"); no struct default so an
	// unset YAML value doesn't override LLM_EFFORT. Recognized: low/medium/high/xhigh/max.
	Effort      string  `yaml:"effort"`
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
