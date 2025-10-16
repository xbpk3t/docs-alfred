package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xbpk3t/docs-alfred/dgh/pkg"
)

// syncCmd represents the sync command
//
//nolint:gochecknoglobals // Required for cobra CLI
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync repository configuration from remote",
	Long: `Sync downloads the repository configuration file from the remote URL
and saves it to the local cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := viper.GetString("config")
		configURL := viper.GetString("url")

		manager := gh.NewManager(configPath, configURL)

		fmt.Printf("Syncing from %s to %s...\n", configURL, configPath) //nolint:forbidigo // CLI output
		if err := manager.Sync(); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Println("Sync completed successfully") //nolint:forbidigo // CLI output

		return nil
	},
}

//nolint:gochecknoinits // Required for cobra CLI initialization
func init() {
	rootCmd.AddCommand(syncCmd)
}
