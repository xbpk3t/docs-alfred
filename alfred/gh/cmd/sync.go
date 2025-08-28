package cmd

import (
	"github.com/spf13/cobra"
)

// SyncJob is the name of the sync job
const SyncJob = "sync"

// getSyncCmd returns the sync command
func getSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "A brief description of your command",
		Run: func(_ *cobra.Command, _ []string) {
			// TODO: Refactor this command to work with new structure
		},
	}
}
