package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/output"
	"github.com/xbpk3t/docs-alfred/pkg/schema"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Execute is the entry point for the docs-cli binary.
func Execute() error {
	carboninit.Setup()
	validator.Setup()

	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	var format string

	rootCmd := &cobra.Command{
		Use:   "docs-cli",
		Short: "Docs workspace consistency commands",
	}

	output.FormatFlag(rootCmd, &format, output.FormatText, []string{output.FormatText, output.FormatJSON}, "Output format: text or json")

	rootCmd.AddCommand(newImagesCmd())
	rootCmd.AddCommand(newBlogCmd())
	rootCmd.AddCommand(newDotfilesCmd())
	rootCmd.AddCommand(newWikiCmd())
	rootCmd.AddCommand(schema.SchemaCmd(rootCmd))

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}
