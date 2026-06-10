package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/workspaceops"
)

const cmdDotfiles = "dotfiles"

type dotfilesFlags struct {
	path    string
	dataDir string
	format  string
}

func newDotfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdDotfiles,
		Short: "Dotfiles consistency commands",
	}

	cmd.AddCommand(newDotfilesCheckCmd())
	cmd.AddCommand(newDotfilesSyncRecordCmd())

	return cmd
}

func newDotfilesCheckCmd() *cobra.Command {
	var flags dotfilesFlags

	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check dotfiles/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesCheck(flags.path, flags.dataDir, flags.format)
		},
	}
	cmd.Flags().StringVar(&flags.path, "path", cmdDotfiles, "dotfiles path")
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")
	addFormatFlag(cmd, &flags.format)

	return cmd
}

func newDotfilesSyncRecordCmd() *cobra.Command {
	var flags dotfilesFlags

	cmd := &cobra.Command{
		Use:   "sync-record",
		Short: "Inspect dotfiles changes for record synchronization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesSyncRecord(flags.path, flags.format)
		},
	}
	cmd.Flags().StringVar(&flags.path, "path", cmdDotfiles, "dotfiles path")
	addFormatFlag(cmd, &flags.format)

	return cmd
}

func runDotfilesCheck(dotfilesPath, dataDir, format string) error {
	result, err := workspaceuc.RunDotfilesCheck(workspaceuc.DotfilesCheckInput{
		DotfilesPath: dotfilesPath,
		DataDir:      dataDir,
	})
	if err != nil {
		return err
	}

	textDetails := fmt.Sprintf("summary: shared=%d df-only=%d gh-only=%d\n",
		result.SharedCount, result.DfOnlyCount, result.GhOnlyCount)
	if err := writeCheckCommandOutput(format, &checkCommandOutput{
		Name:    "dotfiles check",
		Issues:  result.Issues,
		Summary: result.Summary(),
	}, textDetails); err != nil {
		return err
	}

	if workspaceuc.HasIssueErrors(result.Issues) {
		return errors.New("dotfiles check failed")
	}

	return nil
}

func runDotfilesSyncRecord(dotfilesPath, format string) error {
	result := workspaceuc.RunDotfilesSyncRecord(workspaceuc.DotfilesSyncRecordInput{
		DotfilesPath: dotfilesPath,
	})

	if err := writeDotfilesSyncRecordResult(result, format); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("sync-record failed: %s", result.Error)
	}

	return nil
}

func writeDotfilesSyncRecordResult(result *workspaceuc.DotfilesSyncRecordResult, format string) error {
	format, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}
	if format == outputFormatJSON {
		summary := map[string]any{
			"changedFiles": len(result.ChangedFiles),
		}
		if result.Error != "" {
			summary["error"] = result.Error
		}

		return writeCommandOutput(format, &CommandOutput{
			Name:    "dotfiles sync-record",
			OK:      result.OK,
			Summary: summary,
			Results: result.ChangedFiles,
		}, "")
	}

	if !result.OK {
		fmt.Fprintf(os.Stderr, "sync-record failed: %s\n", result.Error)

		return nil
	}

	fmt.Fprintf(os.Stderr, "dotfiles sync record: path=%s changed=%d\n", result.DotfilesPath, len(result.ChangedFiles))
	for _, f := range result.ChangedFiles {
		status := formatDotfilesChangeStatus(f.Status)
		fmt.Fprintf(os.Stderr, "%s %s\n", status, f.Path)
		if f.Gh != nil {
			fmt.Fprintf(os.Stderr, "  gh category=%s files=%s\n", f.Gh.Category, strings.Join(f.Gh.GhFiles, ", "))
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
