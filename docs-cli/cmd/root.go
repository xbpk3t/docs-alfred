package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
)

// Execute is the entry point for the docs-cli binary.
func Execute() error {
	carboninit.Setup()

	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "docs-cli",
		Short: "Docs workspace consistency commands",
	}

	rootCmd.AddCommand(newImagesCmd())
	rootCmd.AddCommand(newBlogCmd())
	rootCmd.AddCommand(newDotfilesCmd())
	rootCmd.AddCommand(newWikiCmd())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}
