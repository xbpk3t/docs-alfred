package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/docs-cli/internal/usecase/workspace"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

const cmdDotfiles = "dotfiles"

func newDotfilesCheckCmd() *cobra.Command {
	var checkPath string

	cmd := &cobra.Command{
		Use:   cmdDotfiles,
		Short: "Check dotfiles/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesCheck(checkPath)
		},
	}
	cmd.Flags().StringVar(&checkPath, cmdDotfiles, cmdDotfiles, "dotfiles path")

	return cmd
}

func newDotfilesSyncPlanCmd() *cobra.Command {
	var syncPlanPath string
	var syncPlanJSON bool

	cmd := &cobra.Command{
		Use:   cmdDotfiles,
		Short: "Plan dotfiles synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesSyncPlan(syncPlanPath, syncPlanJSON)
		},
	}
	cmd.Flags().StringVar(&syncPlanPath, cmdDotfiles, cmdDotfiles, "dotfiles path")
	cmd.Flags().BoolVar(&syncPlanJSON, "json", false, "Print JSON output")

	return cmd
}

func runDotfilesCheck(dotfilesPath string) error {
	result, err := workspaceuc.RunDotfilesCheck(workspaceuc.DotfilesCheckInput{
		DotfilesPath: dotfilesPath,
	})
	if err != nil {
		return err
	}

	checkutil.ReportIssues(result.Issues, "dotfiles check")
	fmt.Fprintf(os.Stderr, "summary: shared=%d df-only=%d gh-only=%d\n",
		result.SharedCount, result.DfOnlyCount, result.GhOnlyCount)
	if checkutil.HasErrors(result.Issues) {
		return errors.New("dotfiles check failed")
	}

	return nil
}

func runDotfilesSyncPlan(dotfilesPath string, jsonOutput bool) error {
	result := workspaceuc.RunDotfilesSyncPlan(workspaceuc.DotfilesSyncPlanInput{
		DotfilesPath: dotfilesPath,
		JSON:         jsonOutput,
	})

	if err := writeDotfilesSyncPlanResult(result, jsonOutput); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("sync-plan failed: %s", result.Error)
	}

	return nil
}

func writeDotfilesSyncPlanResult(result *workspaceuc.DotfilesSyncPlanResult, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		return writeOutput(string(data))
	}

	if !result.OK {
		slog.Error("Sync plan failed", "error", result.Error)

		return nil
	}

	slog.Info("Dotfiles sync plan", "path", result.DotfilesPath, "changed", len(result.ChangedFiles))
	for _, f := range result.ChangedFiles {
		status := formatDotfilesChangeStatus(f.Status)
		slog.Info("Changed file", "status", status, "path", f.Path)
		if f.Gh != nil {
			slog.Info("Gh mapping", "category", f.Gh.Category, "files", strings.Join(f.Gh.GhFiles, ", "))
		}
	}

	return nil
}

func formatDotfilesChangeStatus(status string) string {
	switch status {
	case "M":
		return "modified"
	case "A":
		return "added"
	case "D":
		return "deleted"
	case "??":
		return "untracked"
	default:
		return status
	}
}
