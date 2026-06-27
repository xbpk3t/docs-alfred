package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/ccx/cmd"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/output"
	"github.com/xbpk3t/docs-alfred/pkg/schema"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

func main() {
	carboninit.Setup()
	validator.Setup()

	var format string

	rootCmd := &cobra.Command{
		Use:   "ccx",
		Short: "Claude Code eXtend - Session management tools",
		Long: `ccx provides tools for managing Claude Code sessions,
including session chain walking and session export to wiki.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	output.FormatFlag(rootCmd, &format, output.FormatText, []string{output.FormatText, output.FormatJSON}, "Output format: text or json")

	rootCmd.AddCommand(cmd.NewSessionCmd())
	rootCmd.AddCommand(schema.SchemaCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
