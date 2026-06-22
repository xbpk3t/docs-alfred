package cmd

import (
	"errors"
	"fmt"

	"github.com/creasty/defaults"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
)

type exportConfig struct {
	AI       exportAIConfig `yaml:"ai"`
	WikiRoot string         `default:"wiki" yaml:"wikiRoot"`
}

type exportAIConfig struct {
	APIKey  string `yaml:"apiKey"`
	BaseURL string `default:"https://api.lucc.dev/v1" yaml:"baseUrl"`
	Model   string `default:"deepseek-v4-flash"       yaml:"model"`
}

type exportConfigOverrides struct {
	AI       *ai.ClientConfig
	WikiRoot string
}

func loadExportConfig(path string, overrides exportConfigOverrides) (*exportConfig, error) {
	cfg, err := configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[exportConfig]{
		Path:    path,
		Initial: defaultExportConfig(),
		EnvOverrides: []configutil.EnvOverride{
			{Name: "CCX_WIKI_ROOT", Path: "wikiRoot"},
			{Name: "OPENAI_API_KEY", Path: "ai.apiKey"},
			{Name: "CCX_AI_BASE_URL", Path: "ai.baseUrl"},
			{Name: "CCX_AI_MODEL", Path: "ai.model"},
		},
	})
	if err != nil {
		return nil, formatExportConfigError(err)
	}

	if overrides.WikiRoot != "" {
		cfg.WikiRoot = overrides.WikiRoot
	}
	if overrides.AI != nil {
		if overrides.AI.BaseURL != "" {
			cfg.AI.BaseURL = overrides.AI.BaseURL
		}
		if overrides.AI.Model != "" {
			cfg.AI.Model = overrides.AI.Model
		}
	}

	return &cfg, nil
}

func buildAIConfig(cfg *exportConfig) *ai.ClientConfig {
	resolved := ai.DefaultConfig()
	if cfg != nil {
		if cfg.AI.APIKey != "" {
			resolved.APIKey = cfg.AI.APIKey
		}
		if cfg.AI.BaseURL != "" {
			resolved.BaseURL = cfg.AI.BaseURL
		}
		if cfg.AI.Model != "" {
			resolved.Model = cfg.AI.Model
		}
	}

	return resolved
}

func defaultExportConfig() exportConfig {
	var cfg exportConfig
	defaults.MustSet(&cfg)

	return cfg
}

func formatExportConfigError(err error) error {
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
	default:
		return err
	}
}
