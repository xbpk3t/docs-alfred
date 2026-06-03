package cmd

import (
	"github.com/spf13/cobra"
)

// Execute is the entry point for the docs-cli binary.
func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "docs-cli",
		Short: "CLI for data rendering, validation, and GitHub repo queries",
		Long: `docs-cli: data rendering, data validation, docs-images/dotfiles/blog checking,
and GitHub repository search and sync.`,
	}

	rootCmd.AddCommand(newDataCmd())
	rootCmd.AddCommand(newGhCmd())
	rootCmd.AddCommand(newImagesCmd())
	rootCmd.AddCommand(newDotfilesCmd())
	rootCmd.AddCommand(newBlogCmd())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd.Execute()
}
