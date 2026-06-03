package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newDotfilesCmd creates `dotfiles check` and `dotfiles sync-plan`.
func newDotfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotfiles",
		Short: "Dotfiles consistency commands",
	}

	// dotfiles check
	var checkPath string
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check dotfiles/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesCheck(checkPath)
		},
	}
	checkCmd.Flags().StringVar(&checkPath, "dotfiles", "dotfiles", "dotfiles path")
	cmd.AddCommand(checkCmd)

	// dotfiles sync-plan
	var syncPlanPath string
	var syncPlanJSON bool
	syncPlanCmd := &cobra.Command{
		Use:   "sync-plan",
		Short: "Plan dotfiles synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesSyncPlan(syncPlanPath, syncPlanJSON)
		},
	}
	syncPlanCmd.Flags().StringVar(&syncPlanPath, "dotfiles", "dotfiles", "dotfiles path")
	syncPlanCmd.Flags().BoolVar(&syncPlanJSON, "json", false, "Print JSON output")
	cmd.AddCommand(syncPlanCmd)

	return cmd
}

func runDotfilesCheck(path string) error {
	fmt.Fprintf(os.Stderr, "Checking dotfiles consistency at %q...\n", path)
	// TODO: full port from TS modules/dotfiles/check.ts
	return nil
}

func runDotfilesSyncPlan(path string, asJSON bool) error {
	fmt.Fprintf(os.Stderr, "Planning dotfiles sync at %q (json=%v)...\n", path, asJSON)
	// TODO: full port from TS modules/dotfiles/sync-plan.ts
	return nil
}
