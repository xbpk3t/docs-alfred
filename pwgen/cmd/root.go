package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/pwgen/pkg"
)

var cfgFile string //nolint:gochecknoglobals // Required for cobra CLI

// Execute creates and runs the root command.
func Execute() {
	err := newRootCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var secret string
	var length int
	var uppercase, numbers, punctuation bool
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "pwgen [website]",
		Short: "Generate deterministic passwords based on a secret key and website",
		Long: `pwgen generates deterministic passwords using a secret key and website name.
	The same inputs will always produce the same password.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig(cfgFile)

			secret = resolveSecret(secret, cfg)
			if secret == "" {
				return errors.New("secret key is required (set via --secret, config file, or DEFAULT_PWGEN environment variable)")
			}

			resolveConfigDefaults(cmd, cfg, &length, &uppercase, &numbers, &punctuation, &outputFormat)

			password, err := pwgen.NewGenerator(pwgen.NewConfig(secret, length, uppercase, numbers, punctuation)).Generate(args[0])
			if err != nil {
				return fmt.Errorf("failed to generate password: %w", err)
			}

			result, err := wf.GetFormatter(outputFormat).Format(password)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println

			return nil
		},
	}

	// Config file flag
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pwgen.yaml)")

	// Output format flag
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "plain", "Output format (alfred, plain, raw, rofi)")

	// Password generation flags
	cmd.Flags().StringVarP(&secret, "secret", "s", "", "Secret key for password generation")
	cmd.Flags().IntVarP(&length, "length", "l", 16, "Password length")
	cmd.Flags().BoolVarP(&uppercase, "uppercase", "u", true, "Include uppercase letters")
	cmd.Flags().BoolVarP(&numbers, "numbers", "n", true, "Include numbers")
	cmd.Flags().BoolVarP(&punctuation, "punctuation", "p", false, "Include punctuation characters")

	return cmd
}

func resolveSecret(flagValue string, cfg *koanf.Koanf) string {
	if flagValue != "" {
		return flagValue
	}
	if v := cfg.String("secret"); v != "" {
		return v
	}

	return os.Getenv("DEFAULT_PWGEN")
}

func resolveConfigDefaults(cmd *cobra.Command, cfg *koanf.Koanf, length *int, uppercase, numbers, punctuation *bool, outputFormat *string) {
	if !cmd.Flags().Changed("length") {
		if v := cfg.Int("length"); v > 0 {
			*length = v
		}
	}
	if !cmd.Flags().Changed("output") {
		if v := cfg.String("output"); v != "" {
			*outputFormat = v
		}
	}
	applyBoolCfg(cmd, cfg, "uppercase", uppercase)
	applyBoolCfg(cmd, cfg, "numbers", numbers)
	applyBoolCfg(cmd, cfg, "punctuation", punctuation)
}

func applyBoolCfg(cmd *cobra.Command, cfg *koanf.Koanf, name string, dest *bool) {
	if !cmd.Flags().Changed(name) && cfg.Bool(name) {
		*dest = true
	}
}

// loadConfig reads config from file using koanf. Returns empty koanf instance if no config found.
func loadConfig(cfgFile string) *koanf.Koanf {
	k := koanf.New(".")

	configPath := cfgFile
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return k
		}
		configPath = filepath.Join(home, ".pwgen.yaml")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return k
	}

	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read config file: %v\n", err)
	}

	return k
}
