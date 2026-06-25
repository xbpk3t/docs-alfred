package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/pwgen/internal"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
)

var cfgFile string //nolint:gochecknoglobals // Required for cobra CLI

// Execute creates and runs the root command.
func Execute() {
	carboninit.Setup()
	validator.Setup()

	err := newRootCmd().Execute()
	if err != nil {
		slog.Error("command execution failed", "error", err)
		os.Exit(1)
	}
}

type pwgenConfig struct {
	Secret      string `yaml:"secret"`
	Output      string `default:"plain" yaml:"output"`
	Length      int    `default:"16" yaml:"length"`
	Uppercase   bool   `default:"true" yaml:"uppercase"`
	Numbers     bool   `default:"true" yaml:"numbers"`
	Punctuation bool   `yaml:"punctuation"`
}

func defaultPwgenConfig() pwgenConfig {
	return pwgenConfig{
		Length:    16,
		Uppercase: true,
		Numbers:   true,
		Output:    "plain",
	}
}

func loadPwgenConfig(path string) (pwgenConfig, error) {
	return configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[pwgenConfig]{
		Path:    path,
		Initial: defaultPwgenConfig(),
		EnvOverrides: []configutil.EnvOverride{
			{Name: "DEFAULT_PWGEN", Path: "secret"},
		},
	})
}

func newRootCmd() *cobra.Command {
	var flagSecret string
	var flagLength int
	var flagUppercase, flagNumbers, flagPunctuation bool
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "pwgen [website]",
		Short: "Generate deterministic passwords based on a secret key and website",
		Long: `pwgen generates deterministic passwords using a secret key and website name.
		The same inputs will always produce the same password.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := resolveConfigPath(cfgFile)
			cfg, err := loadPwgenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			cfg = applyFlagOverrides(cmd, cfg, flagSecret, flagLength, flagUppercase, flagNumbers, flagPunctuation, flagOutput)

			if cfg.Secret == "" {
				return errors.New("secret key is required (set via --secret, config file, or DEFAULT_PWGEN environment variable)")
			}

			password, err := pwgen.NewGenerator(pwgen.NewConfig(cfg.Secret, cfg.Length, cfg.Uppercase, cfg.Numbers, cfg.Punctuation)).Generate(args[0])
			if err != nil {
				return fmt.Errorf("failed to generate password: %w", err)
			}

			result, err := wf.GetFormatter(cfg.Output).Format(password)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pwgen.yaml)")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "plain", "Output format (alfred, plain, raw, rofi)")
	cmd.Flags().StringVarP(&flagSecret, "secret", "s", "", "Secret key for password generation")
	cmd.Flags().IntVarP(&flagLength, "length", "l", 16, "Password length")
	cmd.Flags().BoolVarP(&flagUppercase, "uppercase", "u", true, "Include uppercase letters")
	cmd.Flags().BoolVarP(&flagNumbers, "numbers", "n", true, "Include numbers")
	cmd.Flags().BoolVarP(&flagPunctuation, "punctuation", "p", false, "Include punctuation characters")

	return cmd
}

func resolveConfigPath(cfgFile string) string {
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			return ""
		}

		return cfgFile
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	path := filepath.Join(home, ".pwgen.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ""
	}

	return path
}

func applyFlagOverrides(
	cmd *cobra.Command,
	cfg pwgenConfig,
	flagSecret string,
	flagLength int,
	flagUppercase, flagNumbers, flagPunctuation bool,
	flagOutput string,
) pwgenConfig {
	if cmd.Flags().Changed("secret") {
		cfg.Secret = flagSecret
	}
	if cmd.Flags().Changed("length") {
		cfg.Length = flagLength
	}
	if cmd.Flags().Changed("uppercase") {
		cfg.Uppercase = flagUppercase
	}
	if cmd.Flags().Changed("numbers") {
		cfg.Numbers = flagNumbers
	}
	if cmd.Flags().Changed("punctuation") {
		cfg.Punctuation = flagPunctuation
	}
	if cmd.Flags().Changed("output") {
		cfg.Output = flagOutput
	}

	return cfg
}
