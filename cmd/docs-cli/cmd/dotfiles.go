package cmd

import (
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"
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
	// Category-level diff
	dfCats, err := df.LoadDotfilesCategories(dotfilesPath)
	if err != nil {
		return err
	}
	ghCats, err := df.LoadGHCategories(dataDir)
	if err != nil {
		return err
	}
	catDiff := df.DiffCategories(ghCats, dfCats)
	catDiffPtr, err := df.FilterGhOnlyCategories(&catDiff, dataDir)
	if err != nil {
		return err
	}

	// Nix-level diff
	dfNix, err := df.LoadDotfilesNixData(dotfilesPath, df.DefaultScope())
	if err != nil {
		return err
	}
	ghNix, err := df.LoadGHNixData(dataDir)
	if err != nil {
		return err
	}
	falsePkgs, err := df.LoadGHFalsePkgs(dataDir)
	if err != nil {
		return err
	}
	selfBuilt, err := df.LoadSelfBuiltPkgs(filepath.Join(dotfilesPath, "pkgs", "_sources", "generated.json"))
	if err != nil {
		return err
	}
	nixDiff := df.DiffNix(ghNix, dfNix, falsePkgs, selfBuilt)

	// Merge and output
	result := df.MergeResult(catDiffPtr, nixDiff)

	if err := df.WriteOutput(format, result); err != nil {
		return err
	}

	if checkutil.HasErrors(result.Issues) {
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

	return df.WriteDedupOutput(format, dups)
}
