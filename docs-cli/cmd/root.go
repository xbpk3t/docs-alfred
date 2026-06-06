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
		Short: "Docs workspace consistency commands",
	}

	rootCmd.AddCommand(newCheckCmd())
	rootCmd.AddCommand(newSyncPlanCmd())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}
