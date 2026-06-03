package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type imagesCheckFlags struct {
	dataDir     string
	imagesDir   string
	scope       string
	apply       bool
	list        bool
	skipExtra   bool
	skipMissing bool
}

// newImagesCmd creates `images check`.
func newImagesCmd() *cobra.Command {
	var flags imagesCheckFlags

	cmd := &cobra.Command{
		Use:   "images",
		Short: "Docs-images management commands",
	}

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check docs-images against data/gh expectations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImagesCheck(flags)
		},
	}
	checkCmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")
	checkCmd.Flags().StringVar(&flags.imagesDir, "images-dir", "docs-images", "docs-images path")
	checkCmd.Flags().StringVar(&flags.scope, "scope", "", "Only check a path prefix")
	checkCmd.Flags().BoolVar(&flags.apply, "apply", false, "Apply fixes")
	checkCmd.Flags().BoolVar(&flags.list, "list", false, "Print full lists")
	checkCmd.Flags().BoolVar(&flags.skipExtra, "skip-extra-files", false, "Ignore extra files")
	checkCmd.Flags().BoolVar(&flags.skipMissing, "skip-missing", false, "Do not fail on missing expected dirs")

	cmd.AddCommand(checkCmd)

	return cmd
}

func runImagesCheck(flags imagesCheckFlags) error {
	fmt.Fprintf(os.Stderr, "Checking docs-images from data-dir=%q images-dir=%q...\n", flags.dataDir, flags.imagesDir)
	// TODO: full port from TS modules/images/check.ts
	return nil
}
