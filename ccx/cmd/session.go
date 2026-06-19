package cmd

import (
	"github.com/spf13/cobra"
)

// NewSessionCmd creates the session subcommand.
func NewSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Session management commands",
		Long:  `Commands for managing Claude Code sessions, including chain walking and export.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSessionChainCmd())
	cmd.AddCommand(newSessionExportCmd())

	return cmd
}
