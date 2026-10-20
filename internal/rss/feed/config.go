package rss //nolint:revive

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/creasty/defaults"

	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Config 主配置结构.
type Config struct {
	R2Config         R2Config         `yaml:"r2,omitempty"`
	ResendConfig     ResendConfig     `yaml:"resend"`
	NewsletterConfig NewsletterConfig `yaml:"newsletter"`
	RSS              []FeedsDetail    `yaml:"rss"`
	TrnsConfig       TrnsConfig       `yaml:"trns,omitempty"`
	DashboardConfig  DashboardConfig  `yaml:"dashboard"`
	HuntConfig       HuntConfig       `yaml:"hunt,omitempty"`
	FeedConfig       FeedConfig       `yaml:"feed"`
	EnvConfig        EnvConfig        `yaml:"env"`
}

// R2Config configures the R2 upload driver: the bucket and its public
// surface. Credentials come from the environment (R2_AK/R2_SK with
// CF_R2_AK/CF_R2_SK fallback), never from YAML.
type R2Config struct {
	Bucket        string `yaml:"bucket,omitempty"`
	PublicBaseURL string `yaml:"publicBaseUrl,omitempty"`
	Endpoint      string `yaml:"endpoint,omitempty"`
	KeyPrefix     string `yaml:"keyPrefix,omitempty"`
}

// LitterConfig resolves the R2 uploader config. Returns nil when no bucket is
// configured; errors when a bucket is set but the public base URL or
// credentials are missing.
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
	if ak == "" || sk == "" || endpoint == "" {
		return nil, errors.New("r2: credentials missing (need R2_AK with R2_SK/R2_KEY/CF_R2_SK, and an endpoint)")
	}

	return &litter.R2Config{
		Endpoint:        endpoint,
		Bucket:          c.Bucket,
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		PublicBaseURL:   c.PublicBaseURL,
		KeyPrefix:       c.KeyPrefix,
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

// ResendConfig Resend相关配置.
type ResendConfig struct {
	Token  string   `yaml:"token"`
	MailTo []string `yaml:"mailTo"`
}

// NewsletterConfig 新闻通讯配置.
type NewsletterConfig struct {
	Schedule            string `validate:"in:daily,weekly" yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

// FeedConfig Feed相关配置.
type FeedConfig struct {
	Hosts                  []FeedHostRule `yaml:"hosts,omitempty"`
	Timeout                int            `default:"30" validate:"gte:0" yaml:"timeout"`
	MaxTries               int            `default:"3"  validate:"gte:0" yaml:"maxTries"`
	FeedLimit              int            `default:"30" validate:"gte:0" yaml:"feedLimit"`
	Concurrency            int            `default:"10" validate:"gte:0" yaml:"concurrency"`
	HostDefaultConcurrency int            `default:"4"  validate:"gte:0" yaml:"hostDefaultConcurrency"`
}

// FeedHostRule is a per-hostname fetch policy (concurrency, spacing, circuit).
// match is compared case-insensitively to url.Hostname() (exact match).
type FeedHostRule struct {
	CircuitOn5xx  *bool  `yaml:"circuitOn5xx,omitempty"`
	Match         string `yaml:"match"`
	Concurrency   int    `yaml:"concurrency"`
	MinIntervalMs int    `yaml:"minIntervalMs,omitempty"`
}

type DashboardConfig struct {
	FetchFailureReport     FeedFailureReportConfig `yaml:"fetchFailureReport,omitempty"`
	IsShowFetchFailedFeeds bool                    `yaml:"isShowFetchFailedFeeds"`
	FeedDetail             FeedDetailConfig        `yaml:"feedDetail,omitempty"`
}

// FeedDetailConfig Feed详情展示配置.
type FeedDetailConfig struct {
	Enabled     bool `yaml:"enabled"`
	StaleMonths int  `yaml:"staleMonths,omitempty"` // 0 = show all; >0 = only show feeds not updated for N months
}

// FeedsDetail Feed详情.
type FeedsDetail struct {
	Type  string  `yaml:"type"`
	Feeds []Feeds `yaml:"feeds"`
}

type EnvConfig struct {
	Debug bool `yaml:"debug"`
}

// Feeds Feed URL.
type Feeds struct {
	Feed        string  `yaml:"feed"`
	URL         string  `yaml:"url"`
	Des         string  `yaml:"des"`
	LastUpdated string  `yaml:"last_updated,omitempty"`
	PublishFreq string  `yaml:"publish_freq,omitempty"`
	Score       float64 `yaml:"score,omitempty"`
	IsMedia     bool    `yaml:"isMedia,omitempty"`
}

// -- Trns Config --

// TrnsConfig 转写（transcript）主配置.
type TrnsConfig struct {
	Summary         TrnsSummaryConfig         `yaml:"summary,omitempty"`
	TemporaryUpload TrnsTemporaryUploadConfig `yaml:"temporaryUpload,omitempty"`
	DefaultLimit    int                       `default:"10"                     yaml:"defaultLimit,omitempty"`
	Enabled         bool                      `yaml:"enabled,omitempty"`
}

// TrnsSummaryConfig AI 摘要配置.
type TrnsSummaryConfig struct {
	Model    string `yaml:"model,omitempty"`
	Effort   string `yaml:"effort,omitempty"`
	BaseURL  string `yaml:"baseUrl,omitempty"`
	Language string `yaml:"language,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty"`
}

// TrnsTemporaryUploadConfig 临时上传配置.
type TrnsTemporaryUploadConfig struct {
	Driver             string `yaml:"driver,omitempty"` // "litterbox"（默认）| "r2"
	ExpirationDuration string `yaml:"expiration,omitempty"`
	Enabled            bool   `yaml:"enabled,omitempty"`
}

// -- Hunt Config --

// HuntConfig 源发现配置.
type HuntConfig struct {
	Categories      *HuntCategoriesConfig `yaml:"categories,omitempty"`
	ProviderWeights map[string]float64    `yaml:"providerWeights,omitempty"`
	TypeWeights     map[string]float64    `yaml:"typeWeights,omitempty"`
	BlockedDomains  []string              `yaml:"blockedDomains,omitempty"`
	Providers       []string              `yaml:"providers,omitempty"`
	Publish         HuntPublishConfig     `yaml:"publish,omitempty"`
	DefaultMax      int                   `yaml:"defaultMax,omitempty"`
	DefaultPerCat   int                   `yaml:"defaultPerCat,omitempty"`
	DefaultSeed     int                   `yaml:"defaultSeed,omitempty"`
	NewOnly         bool                  `yaml:"newOnly,omitempty"`
}

// HuntPublishConfig hunt 报告发布配置.
type HuntPublishConfig struct {
	Expiration string   `yaml:"expiration,omitempty"`
	Drivers    []string `yaml:"drivers,omitempty"`
	Enabled    bool     `yaml:"enabled,omitempty"`
}

// HuntCategoriesConfig 分类级别覆盖配置.
type HuntCategoriesConfig struct {
	Except []string `yaml:"except,omitempty"`
}

// NewConfig 加载配置文件.
func NewConfig(configFile string) (*Config, error) {
	config, err := configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[Config]{
		Path: configFile,
		EnvOverrides: []configutil.EnvOverride{
			{Name: "RESEND_TOKEN", Path: "resend.token"},
		},
		AfterUnmarshal: func(config *Config) error {
			config.applyDefaults()

			return nil
		},
		Validate: func(config *Config) error {
			return config.Validate()
		},
	})
	if err != nil {
		return nil, wrapConfigLoadError(err)
	}

	return &config, nil
}

func wrapConfigLoadError(err error) error {
	var loadErr *configutil.LoadError
	if !errors.As(err, &loadErr) {
		return err
	}

	switch loadErr.Stage {
	case configutil.StageRead:
		return fmt.Errorf("read config: %w", loadErr.Err)
	case configutil.StageValidate:
		return fmt.Errorf("validate config: %w", loadErr.Err)
	default:
		return fmt.Errorf("unmarshal config: %w", loadErr.Err)
	}
}

func (c *Config) applyDefaults() {
	defaults.MustSet(c)
}

// Validate 验证通用配置（各命令共享的校验）.
func (c *Config) Validate() error {
	if err := validator.Struct(c); err != nil {
		return err
	}
	if err := c.FeedConfig.validateHosts(); err != nil {
		return err
	}

	return nil
}

func (f FeedConfig) validateHosts() error {
	seen := make(map[string]struct{}, len(f.Hosts))
	for i, rule := range f.Hosts {
		match := strings.ToLower(strings.TrimSpace(rule.Match))
		if match == "" {
			return fmt.Errorf("feed.hosts[%d].match is required", i)
		}
		if rule.Concurrency < 0 {
			return fmt.Errorf("feed.hosts[%d].concurrency must be >= 0", i)
		}
		if rule.MinIntervalMs < 0 {
			return fmt.Errorf("feed.hosts[%d].minIntervalMs must be >= 0", i)
		}
		if _, ok := seen[match]; ok {
			return fmt.Errorf("feed.hosts duplicate match %q", match)
		}
		seen[match] = struct{}{}
	}

	return nil
}

// ValidateForSend 额外校验 send 命令所需的 Resend token.
func (c *Config) ValidateForSend() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ResendConfig.Token == "" {
		return errors.New("resend token is required")
	}

	return nil
}
