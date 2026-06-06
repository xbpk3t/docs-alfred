package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// Config holds wiki CLI configuration.
type Config struct {
	Ai   AiConfig   `yaml:"ai"`
	Wiki WikiConfig `yaml:"wiki"`
}

// WikiConfig wiki-specific config.
type WikiConfig struct {
	WikiRoot      string `default:"wiki"                         validate:"required"     yaml:"wikiRoot"`
	GhTopicsURL   string `default:"https://docs.lucc.dev/gh.yml" validate:"required,url" yaml:"ghTopicsURL"`
	Concurrency   int    `default:"5"                            validate:"gte=1"        yaml:"concurrency"`
	PerURLTimeout int    `default:"180"                          validate:"gte=1"        yaml:"perURLTimeout"` // seconds
	MaxRetries    int    `default:"3"                            validate:"gte=0"        yaml:"maxRetries"`
}

// AiConfig AI model configuration.
type AiConfig struct {
	Model   string `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL string `default:"https://api.lucc.dev/v1" validate:"required,url" yaml:"baseUrl"`
}

func defaultConfig() Config {
	var cfg Config
	defaults.MustSet(&cfg)

	return cfg
}

var (
	configFile  string
	wikiRootOpt string
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki [flags] [urls...]",
		Short: "Classify and summarize URLs into wiki knowledge base",
		Long: `Classify and summarize URLs into wiki knowledge base.

Uses AI to classify URLs by content type (video/audio/text), topic path,
and entry type (repo_eval/deep_dive/inbox). Writes structured entries.

Use --inbox to process wiki/inbox.md. Pass URLs as positional args.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			inbox, _ := cmd.Flags().GetBool("inbox")
			if inbox {
				return runInbox(cfg)
			}
			if len(args) == 0 {
				slog.Info("No URLs provided and --inbox not set, doing nothing")

				return nil
			}

			return runURLs(cfg, args)
		},
	}

	cmd.Flags().Bool("inbox", false, "Read URLs from wiki/inbox.md, process, and flush")
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&wikiRootOpt, "wiki-root", "", "Wiki root directory (overrides config)")

	return cmd
}

func loadConfig() (*Config, error) {
	cfg := defaultConfig()

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if wikiRootOpt != "" {
		cfg.Wiki.WikiRoot = wikiRootOpt
	}
	if err := validator.New().Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func newAIConfig(cfg *Config) *ai.ClientConfig {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_AxonHub")
	}

	return &ai.ClientConfig{
		APIKey:  apiKey,
		BaseURL: cfg.Ai.BaseURL,
		Model:   cfg.Ai.Model,
	}
}

func resolveWikiRoot(cfg *Config) string {
	if cfg.Wiki.WikiRoot != "" {
		return cfg.Wiki.WikiRoot
	}

	return "wiki"
}

// Execute is the entry point for the wiki CLI.
// It exits with code 1 on error.
func Execute() {
	rootCmd := newRootCmd()
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("wiki failed", "error", err)
		os.Exit(1)
	}
}
