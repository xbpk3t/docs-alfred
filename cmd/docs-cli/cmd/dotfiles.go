package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

const cmdDotfiles = "dotfiles"

type dotfilesFlags struct {
	path    string
	dataDir string
}

func newDotfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdDotfiles,
		Short: "Dotfiles consistency commands",
	}

	cmd.AddCommand(newDotfilesCheckCmd())

	return cmd
}

func newDotfilesCheckCmd() *cobra.Command {
	var flags dotfilesFlags

	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check dotfiles/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesCheck(flags.path, flags.dataDir, output.GetFormat(cmd))
		},
	}
	cmd.Flags().StringVar(&flags.path, "path", cmdDotfiles, "dotfiles path")
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")

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
