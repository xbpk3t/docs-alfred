package rss

import (
	"errors"
	"fmt"
	"os"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"

	yaml "github.com/goccy/go-yaml"
)

// Config 主配置结构.
type Config struct {
	NtfyConfig       NtfyConfig       `yaml:"ntfy"`
	NewsletterConfig NewsletterConfig `yaml:"newsletter"`
	Feeds            []FeedsDetail    `yaml:"feeds"`
	FeedConfig       FeedConfig       `yaml:"feed"`
	DashboardConfig  DashboardConfig  `yaml:"dashboard"`
	EnvConfig        EnvConfig        `yaml:"env"`
}

// NtfyConfig ntfy 相关配置.
type NtfyConfig struct {
	URL   string `yaml:"url"`
	Topic string `yaml:"topic"`
	Token string `yaml:"token"`
}

// NewsletterConfig 新闻通讯配置.
type NewsletterConfig struct {
	Schedule            string `yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

// FeedConfig Feed相关配置.
type FeedConfig struct {
	Timeout   int `default:"30" yaml:"timeout"`   // HTTP请求超时时间（秒）
	MaxTries  int `default:"3"  yaml:"maxTries"`  // 最大重试次数
	FeedLimit int `default:"30" yaml:"feedLimit"` // Feed数量限制
}

type DashboardConfig struct {
	IsShowFetchFailedFeeds bool `yaml:"isShowFetchFailedFeeds"`
	IsShowFeedDetail       bool `yaml:"isShowFeedDetail"`
}

// FeedsDetail Feed详情.
type FeedsDetail struct {
	Type string  `yaml:"type"`
	URLs []Feeds `yaml:"urls"`
}

type EnvConfig struct {
	Debug bool `default:"true" yaml:"debug"`
}

// Feeds Feed URL.
type Feeds struct {
	Feed string `yaml:"feed"`
	URL  string `yaml:"url"`
	Des  string `yaml:"des"`
}

// NewConfig 加载配置文件.
func NewConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadConfig, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errcode.WithError(errcode.ErrUnmarshalConfig, err)
	}

	if err := config.Validate(); err != nil {
		return nil, errcode.WithError(errcode.ErrValidateConfig, err)
	}

	return &config, nil
}

// Validate 验证配置.
func (c *Config) Validate() error {
	if c.NtfyConfig.URL == "" {
		return errors.New("ntfy url is required")
	}

	if c.NtfyConfig.Topic == "" {
		return errors.New("ntfy topic is required")
	}

	if c.NtfyConfig.Token == "" {
		return errors.New("ntfy token is required")
	}

	if !isValidSchedule(c.NewsletterConfig.Schedule) {
		return fmt.Errorf("invalid schedule: %s", c.NewsletterConfig.Schedule)
	}

	return nil
}

func isValidSchedule(schedule string) bool {
	scheduleTimeRanges := GetScheduleTimeRanges()
	_, exists := scheduleTimeRanges[schedule]

	return exists
}
