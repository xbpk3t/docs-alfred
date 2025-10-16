package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/pwgen/pkg"
)

var cfgFile string //nolint:gochecknoglobals // Required for cobra CLI

// rootCmd represents the base command.
//
//nolint:gochecknoglobals // Required for cobra CLI
var rootCmd = &cobra.Command{
	Use:   "pwgen [website]",
	Short: "Generate deterministic passwords based on a secret key and website",
	Long: `pwgen generates deterministic passwords using a secret key and website name.
The same inputs will always produce the same password.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		website := args[0]

		// Get configuration from viper
		secretKey := viper.GetString("secret")
		if secretKey == "" {
			return errors.New("secret key is required (set via --secret, config file, or PWGEN_SECRET environment variable)")
		}

		length := viper.GetInt("length")
		uppercase := viper.GetBool("uppercase")
		numbers := viper.GetBool("numbers")
		punctuation := viper.GetBool("punctuation")
		outputFormat := viper.GetString("output")

		// Create password generator config
		config := pwgen.NewConfig(secretKey, length, uppercase, numbers, punctuation)
		generator := pwgen.NewGenerator(config)

		// Generate password
		password, err := generator.Generate(website)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}

		// Format output
		formatter := wf.GetFormatter(outputFormat)
		result, err := formatter.Format(password)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}

		fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

//nolint:gochecknoinits // Required for cobra CLI initialization
func init() {
	cobra.OnInitialize(initConfig)

	// Config file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pwgen.yaml)")

	// Output format flag
	rootCmd.PersistentFlags().StringP("output", "o", "plain", "Output format (alfred, plain, raw, rofi)")
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Password generation flags
	rootCmd.Flags().StringP("secret", "s", "", "Secret key for password generation")
	_ = viper.BindPFlag("secret", rootCmd.Flags().Lookup("secret"))

	rootCmd.Flags().IntP("length", "l", 16, "Password length")
	_ = viper.BindPFlag("length", rootCmd.Flags().Lookup("length"))

	rootCmd.Flags().BoolP("uppercase", "u", true, "Include uppercase letters")
	_ = viper.BindPFlag("uppercase", rootCmd.Flags().Lookup("uppercase"))

	rootCmd.Flags().BoolP("numbers", "n", true, "Include numbers")
	_ = viper.BindPFlag("numbers", rootCmd.Flags().Lookup("numbers"))

	rootCmd.Flags().BoolP("punctuation", "p", false, "Include punctuation characters")
	_ = viper.BindPFlag("punctuation", rootCmd.Flags().Lookup("punctuation"))

	// Set default values
	viper.SetDefault("length", 16)
	viper.SetDefault("uppercase", true)
	viper.SetDefault("numbers", true)
	viper.SetDefault("punctuation", false)
	viper.SetDefault("output", "plain")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".pwgen" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".pwgen")
	}

	// Environment variables
	viper.SetEnvPrefix("PWGEN")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
