package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/docs-cli/internal/usecase/workspace"
)

type imagesCheckFlags struct {
	dataDir     string
	imagesDir   string
	apply       bool
	list        bool
	skipExtra   bool
	skipMissing bool
}

func newImagesCheckCmd() *cobra.Command {
	var flags imagesCheckFlags
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Check docs-images against data/gh expectations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImagesCheck(workspaceuc.ImagesCheckInput{
				DataDir:     flags.dataDir,
				ImagesDir:   flags.imagesDir,
				Apply:       flags.apply,
				List:        flags.list,
				SkipExtra:   flags.skipExtra,
				SkipMissing: flags.skipMissing,
			})
		},
	}

	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")
	cmd.Flags().StringVar(&flags.imagesDir, "images-dir", "docs-images", "docs-images path")
	cmd.Flags().BoolVar(&flags.apply, "apply", false, "Apply fixes")
	cmd.Flags().BoolVar(&flags.list, "list", false, "Print full lists")
	cmd.Flags().BoolVar(&flags.skipExtra, "skip-extra-files", false, "Ignore extra files")
	cmd.Flags().BoolVar(&flags.skipMissing, "skip-missing", false, "Do not fail on missing expected dirs")

	return cmd
}

func runImagesCheck(input workspaceuc.ImagesCheckInput) error {
	fmt.Fprintf(os.Stderr, "Checking docs-images from data-dir=%q images-dir=%q...\n", input.DataDir, input.ImagesDir)

	result, err := workspaceuc.RunImagesCheck(input)
	if err != nil {
		return err
	}

	report := workspaceuc.FormatImagesReport(result, input)
	fmt.Fprint(os.Stderr, report)

	if len(result.Errors) > 0 {
		return fmt.Errorf("images check found %d errors", len(result.Errors))
	}
	if len(result.MissingDirs) > 0 && !input.SkipMissing {
		return fmt.Errorf("images check: %d missing expected dirs", len(result.MissingDirs))
	}

	return nil
}
