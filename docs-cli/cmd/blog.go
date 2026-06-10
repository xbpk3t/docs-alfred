package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/workspaceops"
)

type blogCheckFlags struct {
	dataDir string
	blogDir string
	format  string
}

func newBlogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blog",
		Short: "Blog consistency commands",
	}

	cmd.AddCommand(newBlogCheckCmd())

	return cmd
}

func newBlogCheckCmd() *cobra.Command {
	var flags blogCheckFlags

	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check blog/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlogCheck(flags.dataDir, flags.blogDir, flags.format)
		},
	}
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", "data/gh", "data/gh path")
	cmd.Flags().StringVar(&flags.blogDir, "blog-dir", "blog", "blog path")
	addFormatFlag(cmd, &flags.format)

	return cmd
}

func runBlogCheck(dataDir, blogDir, format string) error {
	result, err := workspaceuc.RunBlogCheck(workspaceuc.BlogCheckInput{
		DataDir: dataDir,
		BlogDir: blogDir,
	})
	if err != nil {
		return err
	}

	textDetails := fmt.Sprintf("summary: data/gh types=%d blog dirs=%d\n", result.GHTypes, result.BlogDirs)
	if err := writeCheckCommandOutput(format, &checkCommandOutput{
		Name:    "blog check",
		Issues:  result.Issues,
		Summary: result.Summary(),
	}, textDetails); err != nil {
		return err
	}

	if workspaceuc.HasIssueErrors(result.Issues) {
		return errors.New("blog check failed")
	}

	return nil
}
