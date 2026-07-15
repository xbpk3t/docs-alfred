package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

type imagesCheckFlags struct {
	dataDir   string
	imagesDir string
	list      bool
	skipExtra bool
}

func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Docs image consistency commands",
	}

	cmd.AddCommand(newImagesCheckCmd())

	return cmd
}

func newImagesCheckCmd() *cobra.Command {
	var flags imagesCheckFlags
	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check docs-images against data/gh expectations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImagesCheck(workspaceuc.ImagesCheckInput{
				DataDir:   flags.dataDir,
				ImagesDir: flags.imagesDir,
				List:      flags.list,
				SkipExtra: flags.skipExtra,
			}, output.GetFormat(cmd))
		},
	}

	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")
	cmd.Flags().StringVar(&flags.imagesDir, "images-dir", "docs-images", "docs-images path")
	cmd.Flags().BoolVar(&flags.list, "list", false, "Print full lists")
	cmd.Flags().BoolVar(&flags.skipExtra, "skip-extra-files", false, "Ignore extra files")
	return cmd
}

func runImagesCheck(input workspaceuc.ImagesCheckInput, format string) error {
	fmt.Fprintf(os.Stderr, "Checking docs-images from data-dir=%q images-dir=%q...\n", input.DataDir, input.ImagesDir)

	result, err := workspaceuc.RunImagesCheck(input)
	if err != nil {
		return err
	}

	return writeImagesCheckResult("images check", result, input, format, nil)
}

func writeImagesCheckResult(
	name string,
	result *workspaceuc.ImagesCheckResult,
	input workspaceuc.ImagesCheckInput,
	format string,
	actions []string,
) error {
	issues := result.Issues(input)
	textDetails := workspaceuc.FormatImagesDetails(result, input)
	if err := writeCheckCommandOutput(format, &checkCommandOutput{
		Name:    name,
		Issues:  issues,
		Summary: result.Summary(),
		Actions: actions,
	}, textDetails); err != nil {
		return err
	}

	if checkutil.HasErrors(issues) {
		return fmt.Errorf("%s failed", name)
	}

	return nil
}
