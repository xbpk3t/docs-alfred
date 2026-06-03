package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/dotfiles"
)

const cmdCheck = "check"

func newDotfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotfiles",
		Short: "Dotfiles consistency commands",
	}

	var checkPath string
	checkCmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check dotfiles/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := dotfiles.RunCheck(checkPath, "data/gh")
			if err != nil {
				return err
			}
			result.Report("dotfiles check")
			if result.HasErrors() {
				return errors.New("dotfiles check failed")
			}

			return nil
		},
	}
	checkCmd.Flags().StringVar(&checkPath, "dotfiles", "dotfiles", "dotfiles path")
	cmd.AddCommand(checkCmd)

	var syncPlanPath string
	var syncPlanJSON bool
	syncPlanCmd := &cobra.Command{
		Use:   "sync-plan",
		Short: "Plan dotfiles synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := dotfiles.RunSyncPlan(dotfiles.SyncPlanOptions{
				DotfilesPath: syncPlanPath,
				JSON:         syncPlanJSON,
			})
			result.PrintResult(syncPlanJSON)
			if !result.OK {
				return fmt.Errorf("sync-plan failed: %s", result.Error)
			}

			return nil
		},
	}
	syncPlanCmd.Flags().StringVar(&syncPlanPath, "dotfiles", "dotfiles", "dotfiles path")
	syncPlanCmd.Flags().BoolVar(&syncPlanJSON, "json", false, "Print JSON output")
	cmd.AddCommand(syncPlanCmd)

	return cmd
}
