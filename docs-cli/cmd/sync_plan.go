package cmd

import "github.com/spf13/cobra"

func newSyncPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync-plan",
		Short: "Plan docs workspace synchronization",
	}

	cmd.AddCommand(newDotfilesSyncPlanCmd())

	return cmd
}
