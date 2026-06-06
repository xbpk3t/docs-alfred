package cmd

import (
	"github.com/spf13/cobra"
)

// Execute is the entry point for the docs-cli binary.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "docs-cli",
		Short: "CLI for data rendering, validation, and GitHub repo queries",
		Long: `docs-cli: data rendering, data validation, docs-images/dotfiles/blog checking,
and GitHub repository search and sync.`,
	}

	rootCmd.AddCommand(newDataCmd())
	rootCmd.AddCommand(newCatalogCmd())
	rootCmd.AddCommand(newWorkspaceCmd())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}
