package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
	"github.com/xbpk3t/docs-alfred/workflow/gh/pkg"
)

var cfgFile string //nolint:gochecknoglobals // Required for cobra CLI

// rootCmd represents the base command.
//
//nolint:gochecknoglobals // Required for cobra CLI
var rootCmd = &cobra.Command{
	Use:   "gh [query]",
	Short: "Search GitHub repositories from your configuration",
	Long: `gh searches through your configured GitHub repositories.
It automatically syncs the configuration from remote if not found locally.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get configuration from viper
		configPath := viper.GetString("config")
		configURL := viper.GetString("url")
		docsURL := viper.GetString("docs")
		outputFormat := viper.GetString("output")

		// Create manager
		manager := gh.NewManager(configPath, configURL)

		// Load configuration (will auto-sync if not found)
		if err := manager.Load(); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get query
		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		// Filter repositories
		repos := manager.Filter(query)

		// Format output
		formatter := wf.GetFormatter(outputFormat)

		switch outputFormat {
		case "alfred":
			result, err := formatAlfredOutput(repos, docsURL)
			if err != nil {
				return err
			}
			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println
		case "plain":
			result := formatPlainOutput(repos, docsURL)
			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println
		case "raw":
			result, err := formatter.Format(repos)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}
			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println
		case "rofi":
			result, err := formatRofiOutput(repos)
			if err != nil {
				return err
			}
			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println
		default:
			result := formatPlainOutput(repos, docsURL)
			fmt.Println(result) //nolint:forbidigo // CLI output requires fmt.Println
		}

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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	// Output format flag
	rootCmd.PersistentFlags().StringP("output", "o", "plain", "Output format (alfred, plain, raw, rofi)")
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Config URL flag
	rootCmd.Flags().StringP("url", "u", gh.DefaultConfigURL, "Config file URL for syncing")
	_ = viper.BindPFlag("url", rootCmd.Flags().Lookup("url"))

	// Docs URL flag
	rootCmd.Flags().StringP("docs", "d", "https://docs.lucc.dev", "Docs base URL")
	_ = viper.BindPFlag("docs", rootCmd.Flags().Lookup("docs"))

	// Set default values
	viper.SetDefault("config", gh.DefaultConfigPath)
	viper.SetDefault("url", gh.DefaultConfigURL)
	viper.SetDefault("docs", "https://docs.lucc.dev")
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

		// Search config in home directory with name ".gh" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gh")
	}

	// Environment variables
	viper.SetEnvPrefix("GH")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// formatAlfredOutput formats repositories for Alfred.
func formatAlfredOutput(repos gh.Repos, docsURL string) (string, error) {
	var items []wf.AlfredItem

	for _, repo := range repos {
		item := wf.AlfredItem{
			Title:    repo.FullName(),
			Subtitle: repo.GetDes(),
			Arg:      repo.GetURL(),
			Valid:    true,
		}

		// Add icon based on repository properties
		switch {
		case repo.HasQs() && repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: gh.IconQsDoc}
		case repo.HasQs():
			item.Icon = &wf.AlfredIcon{Path: gh.IconQs}
		case repo.Doc != "":
			item.Icon = &wf.AlfredIcon{Path: gh.IconDoc}
		default:
			item.Icon = &wf.AlfredIcon{Path: gh.IconSearch}
		}

		// Add modifiers
		item.Mods = make(map[string]*wf.AlfredMod)

		// Cmd modifier - open docs
		if repo.Doc != "" {
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
			item.Mods["cmd"] = &wf.AlfredMod{
				Valid:    true,
				Arg:      docURL,
				Subtitle: "Open documentation",
			}
		}

		items = append(items, item)
	}

	alfredOutput := wf.AlfredOutput{Items: items}
	formatter := wf.GetFormatter("alfred")

	return formatter.Format(alfredOutput)
}

// formatPlainOutput formats repositories as plain text.
//
//nolint:revive // Complexity is acceptable for formatting function
func formatPlainOutput(repos gh.Repos, docsURL string) string {
	var result strings.Builder

	for i, repo := range repos {
		if i > 0 {
			result.WriteString("\n\n")
		}

		result.WriteString(fmt.Sprintf("repo: %s\n", repo.GetURL()))
		if repo.GetDes() != "" {
			result.WriteString(fmt.Sprintf("desc: %s\n", repo.GetDes()))
		}
		if repo.Doc != "" {
			docURL := fmt.Sprintf("%s/#/%s", docsURL, repo.Doc)
			result.WriteString(fmt.Sprintf("doc: %s\n", repo.Doc))
			result.WriteString(fmt.Sprintf("docs: %s\n", docURL))
		}
		if repo.Type != "" {
			typeInfo := repo.Type
			if repo.Tag != "" {
				typeInfo = fmt.Sprintf("%s#%s", repo.Tag, repo.Type)
			}
			result.WriteString(fmt.Sprintf("type: %s\n", typeInfo))
		}
	}

	return result.String()
}

// formatRofiOutput formats repositories for Rofi.
func formatRofiOutput(repos gh.Repos) (string, error) {
	var lines []string
	for _, repo := range repos {
		line := repo.FullName()
		if repo.GetDes() != "" {
			line = fmt.Sprintf("%s - %s", line, repo.GetDes())
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}
