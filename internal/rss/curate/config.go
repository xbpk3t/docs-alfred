// Package curate implements the rss2nl `curate` subcommand: turning an external
// seed list (bestblogs roster) into candidate rss2nl.yml entries. The pipeline
// is deliberately split into a deterministic layer (fetch / frequency /
// dedupe / assemble) and a MAF-orchestrated AI layer (resolve / classify /
// curator). Only preview artifacts are produced — never an append / send / commit.
package curate

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// Config holds the curate rules. Defaults live here (not scattered in callers).
// It is loaded from an optional --config file; env vars take precedence for
// model / endpoint so the CLI stays source-repo-agnostic.
type Config struct {
	KnownAliases      map[string]string  `yaml:"knownAliases,omitempty"`
	Model             string             `yaml:"model,omitempty"`
	BaseURL           string             `yaml:"baseUrl,omitempty"`
	HostRules         []rss.FeedHostRule `yaml:"hostRules,omitempty"`
	MaxCandidates     int                `yaml:"maxCandidates,omitempty"`
	LastMaxDays       int                `yaml:"lastMaxDays"`
	Posts90DMin       int                `yaml:"posts90dMin"`
	ClassifyChunkSize int                `yaml:"classifyChunkSize,omitempty"`
	CurateConcurrency int                `yaml:"curateConcurrency,omitempty"`
	AITimeout         time.Duration      `yaml:"aiTimeout,omitempty"`
	AIRetries         int                `yaml:"aiRetries,omitempty"`
	AIRetryInitialMs  int                `yaml:"aiRetryInitialMs,omitempty"`
	AIRetryBudget     time.Duration      `yaml:"aiRetryBudget,omitempty"`
}

// defaultValue helpers mirror pkg/ai defaults so new Config() is env-driven.
const (
	defaultPosts90DMin    = 10
	defaultLastMaxDays    = 60
	defaultMaxCandidates  = 60
	defaultClassifyChunk  = 20
	defaultCurateConcur   = 8
	defaultAITimeout      = 60 * time.Second
	defaultAIRetries      = 3
	defaultAIRetryBackoff = 2 * time.Second
	defaultAIRetryBudget  = 6 * time.Minute
)

// Defaults returns a Config with built-in defaults applied.
func Defaults() *Config {
	return &Config{
		Posts90DMin:       defaultPosts90DMin,
		LastMaxDays:       defaultLastMaxDays,
		MaxCandidates:     defaultMaxCandidates,
		ClassifyChunkSize: defaultClassifyChunk,
		CurateConcurrency: defaultCurateConcur,
		AITimeout:         defaultAITimeout,
		AIRetries:         defaultAIRetries,
		AIRetryInitialMs:  int(defaultAIRetryBackoff / time.Millisecond),
		AIRetryBudget:     defaultAIRetryBudget,
		Model:             os.Getenv("LLM_MODEL"),
		BaseURL:           os.Getenv("OPENAI_BASE_URL"),
	}
}

// LoadConfig merges built-in defaults with an optional YAML rules file. A
// missing file yields just the defaults (all flags/aliases stay optional).
func LoadConfig(path string) (*Config, error) {
	cfg := Defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read curate config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse curate config %s: %w", path, err)
		}
	}
	applyGateDefaults(cfg)
	applyModelDefaults(cfg)

	return cfg, nil
}

// applyGateDefaults forces nonzero values for the tuning knobs so a config file
// that explicitly omits one (or sets a meaningless 0) still gets a sensible gate.
func applyGateDefaults(cfg *Config) {
	if cfg.Posts90DMin == 0 {
		cfg.Posts90DMin = defaultPosts90DMin
	}
	if cfg.LastMaxDays == 0 {
		cfg.LastMaxDays = defaultLastMaxDays
	}
	if cfg.MaxCandidates == 0 {
		cfg.MaxCandidates = defaultMaxCandidates
	}
	if cfg.ClassifyChunkSize == 0 {
		cfg.ClassifyChunkSize = defaultClassifyChunk
	}
	if cfg.CurateConcurrency == 0 {
		cfg.CurateConcurrency = defaultCurateConcur
	}
	if cfg.AITimeout <= 0 {
		cfg.AITimeout = defaultAITimeout
	}
	if cfg.AIRetries == 0 {
		cfg.AIRetries = defaultAIRetries
	}
	if cfg.AIRetryInitialMs == 0 {
		cfg.AIRetryInitialMs = int(defaultAIRetryBackoff / time.Millisecond)
	}
	if cfg.AIRetryBudget <= 0 {
		cfg.AIRetryBudget = defaultAIRetryBudget
	}
}

// applyModelDefaults falls back to pkg/ai's env defaults exactly like the rest
// of the repo, so MAF talks to the same endpoint as langchain does.
func applyModelDefaults(cfg *Config) {
	if cfg.Model == "" {
		cfg.Model = ai.DefaultConfig().Model
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = ai.DefaultConfig().BaseURL
	}
}
