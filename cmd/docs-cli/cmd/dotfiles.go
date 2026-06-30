package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	df "github.com/xbpk3t/docs-alfred/internal/docs/dotfiles"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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
	cmd.AddCommand(newDotfilesDedupCmd())

	return cmd
}

// ---------------------------------------
// dotfiles check — category-level + nix-level diff
// ---------------------------------------

func newDotfilesCheckCmd() *cobra.Command {
	var flags dotfilesFlags

	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check dotfiles/data consistency (categories + nix packages)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesFullCheck(flags.path, flags.dataDir, output.GetFormat(cmd))
		},
	}
	cmd.Flags().StringVar(&flags.path, "path", cmdDotfiles, "dotfiles path")
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")

	return cmd
}

// runDotfilesCheck is an alias used by tests.
var runDotfilesCheck = runDotfilesFullCheck

func runDotfilesFullCheck(dotfilesPath, dataDir, format string) error {
	// Category-level check
	catResult, err := workspaceuc.RunDotfilesCheck(workspaceuc.DotfilesCheckInput{
		DotfilesPath: dotfilesPath,
		DataDir:      dataDir,
	})
	if err != nil {
		return err
	}

	// Nix-level diff
	nixDiff, err := df.RunNixDiff(dataDir, dotfilesPath, df.DefaultScope())
	if err != nil {
		return err
	}

	allIssues := make([]checkutil.Issue, 0, len(catResult.Issues)+len(nixDiff.Issues))
	allIssues = append(allIssues, catResult.Issues...)
	allIssues = append(allIssues, nixDiff.Issues...)

	catSum := catResult.Summary()
	nixSum := nixDiff.Summary()

	textDetails := fmt.Sprintf(
		"categories shared=%d df-only=%d gh-only=%d | nix gh-only=%d df-only=%d\n",
		catSum["shared"], catSum["dfOnly"], catSum["ghOnly"],
		nixSum["ghOnly"], nixSum["dfOnly"],
	)

	combined := make(map[string]any)
	for k, v := range catSum {
		combined[k] = v
	}
	for k, v := range nixSum {
		combined["nix"+k] = v
	}

	if err := writeCheckCommandOutput(format, &checkCommandOutput{
		Name:    "dotfiles check",
		Issues:  allIssues,
		Summary: combined,
	}, textDetails); err != nil {
		return err
	}

	if workspaceuc.HasIssueErrors(allIssues) {
		return errors.New("dotfiles check failed")
	}

	return nil
}

// ---------------------------------------
// dotfiles dedup — find duplicate nix refs by category
// ---------------------------------------

func newDotfilesDedupCmd() *cobra.Command {
	var flags dotfilesFlags

	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Find nix packages referenced in multiple dotfiles (potential duplicates)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDotfilesDedup(flags.path, output.GetFormat(cmd))
		},
	}
	cmd.Flags().StringVar(&flags.path, "path", cmdDotfiles, "dotfiles path")

	return cmd
}

func runDotfilesDedup(dotfilesPath, format string) error {
	dups, err := df.DedupRef(dotfilesPath, df.DefaultScope())
	if err != nil {
		return err
	}

	if len(dups) == 0 {
		slog.Info("no duplicates found")
		return nil
	}

	// Build issues for each duplicated package
	var issues []checkutil.Issue
	for pkg, files := range dups {
		sort.Strings(files)
		issues = append(issues, checkutil.Issue{
			File:     pkg,
			Severity: checkutil.SeverityWarn,
			Message:  fmt.Sprintf("pkgs.%s referenced in multiple files: %s", pkg, strings.Join(files, ", ")),
		})
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"name":    "dotfiles dedup",
			"total":   len(dups),
			"results": issues,
		})
	}

	slog.Info(fmt.Sprintf("found %d duplicate package references:", len(dups)))
	for _, iss := range issues {
		slog.Info(fmt.Sprintf("  %s", iss.Message))
	}

	return nil
}
