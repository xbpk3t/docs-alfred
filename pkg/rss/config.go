package rss

import (
	"errors"
	"fmt"
	"os"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"

	"github.com/goccy/go-yaml"
)

// Config 主配置结构
type Config struct {
	ResendConfig     ResendConfig     `yaml:"resend"`
	NewsletterConfig NewsletterConfig `yaml:"newsletter"`
	Feeds            []FeedsDetail    `yaml:"feeds"`
	FeedConfig       FeedConfig       `yaml:"feed"`
	EnvConfig        EnvConfig        `yaml:"env"`
	DashboardConfig  DashboardConfig  `yaml:"dashboard"`
}

// ResendConfig Resend相关配置
type ResendConfig struct {
	Token  string   `yaml:"token"`
	MailTo []string `yaml:"mailTo"`
}

// NewsletterConfig 新闻通讯配置
type NewsletterConfig struct {
	Schedule            string `yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

// FeedConfig Feed相关配置
type FeedConfig struct {
	Timeout   int `yaml:"timeout" default:"30"`   // HTTP请求超时时间（秒）
	MaxTries  int `yaml:"maxTries" default:"3"`   // 最大重试次数
	FeedLimit int `yaml:"feedLimit" default:"30"` // Feed数量限制
}

type DashboardConfig struct {
	IsShowFetchFailedFeeds bool `yaml:"isShowFetchFailedFeeds"`
	IsShowTypeStats        bool `yaml:"isShowTypeStats"`
	IsShowFeedDetail       bool `yaml:"isShowFeedDetail"`
}

// FeedsDetail Feed详情
type FeedsDetail struct {
	Type string  `yaml:"type"`
	Urls []Feeds `yaml:"urls"`
}

type EnvConfig struct {
	Debug bool `yaml:"debug" default:"true"`
}

// Feeds Feed URL
type Feeds struct {
	Feed string `yaml:"feed"`
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
	Des  string `yaml:"des"`
}

// NewConfig 加载配置文件
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

// Validate 验证配置
func (c *Config) Validate() error {
	if c.ResendConfig.Token == "" {
		return errors.New("resend token is required")
	}

	if !isValidSchedule(c.NewsletterConfig.Schedule) {
		return fmt.Errorf("invalid schedule: %s", c.NewsletterConfig.Schedule)
	}

	return nil
}

func isValidSchedule(schedule string) bool {
	_, exists := scheduleTimeRanges[schedule]
	return exists
}
