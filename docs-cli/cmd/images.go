package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/images"
)

const cmdImages = "images"

type imagesCheckFlags struct {
	dataDir     string
	imagesDir   string
	scope       string
	apply       bool
	list        bool
	skipExtra   bool
	skipMissing bool
}

func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdImages,
		Short: "Docs-images management commands",
	}

	var flags imagesCheckFlags
	checkCmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check docs-images against data/gh expectations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := images.CheckConfig{
				DataDir:     flags.dataDir,
				ImagesDir:   flags.imagesDir,
				Scope:       flags.scope,
				Apply:       flags.apply,
				List:        flags.list,
				SkipExtra:   flags.skipExtra,
				SkipMissing: flags.skipMissing,
			}

			return runImagesCheck(cfg)
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

func runImagesCheck(cfg images.CheckConfig) error {
	fmt.Fprintf(os.Stderr, "Checking docs-images from data-dir=%q images-dir=%q...\n", cfg.DataDir, cfg.ImagesDir)

	result, err := images.RunImagesCheck(cfg)
	if err != nil {
		return err
	}

	result.Report(cfg)

	if len(result.Errors) > 0 {
		return fmt.Errorf("images check found %d errors", len(result.Errors))
	}
	if len(result.MissingDirs) > 0 && !cfg.SkipMissing {
		return fmt.Errorf("images check: %d missing expected dirs", len(result.MissingDirs))
	}

	return nil
}
