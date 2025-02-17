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
	Resend     ResendConfig     `yaml:"resend"`
	Newsletter NewsletterConfig `yaml:"newsletter"`
	Feeds      []FeedsDetail    `yaml:"feeds"`
	Feed       FeedConfig       `yaml:"feed"`
}

// ResendConfig Resend相关配置
type ResendConfig struct {
	Token string `yaml:"token"`
}

// NewsletterConfig 新闻通讯配置
type NewsletterConfig struct {
	Schedule            string `yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

// FeedConfig Feed相关配置
type FeedConfig struct {
	MaxTries  int `yaml:"maxTries"`
	FeedLimit int `yaml:"feedLimit"`
}

// FeedsDetail Feed详情
type FeedsDetail struct {
	Type string  `yaml:"type"`
	Urls []Feeds `yaml:"urls"`
}

// Feeds Feed URL
type Feeds struct {
	Feed string `yaml:"feed"`
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
	if c.Resend.Token == "" {
		return errors.New("resend token is required")
	}

	if !isValidSchedule(c.Newsletter.Schedule) {
		return fmt.Errorf("invalid schedule: %s", c.Newsletter.Schedule)
	}

	return nil
}

func isValidSchedule(schedule string) bool {
	_, exists := scheduleTimeRanges[schedule]
	return exists
}
