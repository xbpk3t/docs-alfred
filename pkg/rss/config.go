package rss

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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

// NewConfig 创建新的配置
func NewConfig(cfgFile string) (*Config, error) {
	fx, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(fx, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
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
