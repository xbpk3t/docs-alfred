package rss //nolint:revive

import (
	"errors"
	"fmt"

	"github.com/creasty/defaults"

	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Config 主配置结构.
type Config struct {
	WikiConfig       WikiConfig       `yaml:"wiki,omitempty"`
	ResendConfig     ResendConfig     `yaml:"resend"`
	NewsletterConfig NewsletterConfig `yaml:"newsletter"`
	RSS              []FeedsDetail    `yaml:"rss"`
	TrnsConfig       TrnsConfig       `yaml:"trns,omitempty"`
	DashboardConfig  DashboardConfig  `yaml:"dashboard"`
	HuntConfig       HuntConfig       `yaml:"hunt,omitempty"`
	FeedConfig       FeedConfig       `yaml:"feed"`
	EnvConfig        EnvConfig        `yaml:"env"`
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
	Timeout   int `default:"30" validate:"gte:0" yaml:"timeout"`   // HTTP请求超时时间（秒）
	MaxTries  int `default:"3"  validate:"gte:0" yaml:"maxTries"`  // 最大重试次数
	FeedLimit int `default:"30" validate:"gte:0" yaml:"feedLimit"` // Feed数量限制
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
	Asr             TrnsAsrConfig             `yaml:"asr,omitempty"`
	TemporaryUpload TrnsTemporaryUploadConfig `yaml:"temporaryUpload,omitempty"`
	DefaultLimit    int                       `default:"10"                     yaml:"defaultLimit,omitempty"`
	Enabled         bool                      `yaml:"enabled,omitempty"`
}

// TrnsAsrConfig ASR（自动语音识别）配置.
type TrnsAsrConfig struct {
	Language string `yaml:"language,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty"`
}

// TrnsSummaryConfig AI 摘要配置.
type TrnsSummaryConfig struct {
	Model    string `yaml:"model,omitempty"`
	BaseURL  string `yaml:"baseUrl,omitempty"`
	Language string `yaml:"language,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty"`
}

// TrnsTemporaryUploadConfig 临时上传配置（Litterbox）.
type TrnsTemporaryUploadConfig struct {
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

// -- Wiki Config --

// WikiConfig Wiki 知识库配置.
type WikiConfig struct {
	WikiRootDir  string       `yaml:"wikiRootDir,omitempty"`
	GhTopicsPath string       `yaml:"ghTopicsPath,omitempty"`
	PendingPath  string       `yaml:"pendingPath,omitempty"`
	Ai           WikiAiConfig `yaml:"ai,omitempty"`
}

// WikiAiConfig Wiki AI 配置（fallback 到 trns.summary 的 model/baseUrl）.
type WikiAiConfig struct {
	Model   string `yaml:"model,omitempty"`
	BaseURL string `yaml:"baseUrl,omitempty"`
}

// AiModelForWiki returns the effective model for wiki AI.
// Falls back to trns.summary.model if wiki.ai.model is empty.
func (c *Config) AiModelForWiki() string {
	if c.WikiConfig.Ai.Model != "" {
		return c.WikiConfig.Ai.Model
	}

	return c.TrnsConfig.Summary.Model
}

// AiBaseURLForWiki returns the effective base URL for wiki AI.
// Falls back to trns.summary.baseUrl if wiki.ai.baseUrl is empty.
func (c *Config) AiBaseURLForWiki() string {
	if c.WikiConfig.Ai.BaseURL != "" {
		return c.WikiConfig.Ai.BaseURL
	}

	return c.TrnsConfig.Summary.BaseURL
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
