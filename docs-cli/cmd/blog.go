package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/docs-cli/internal/usecase/workspace"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func newBlogCmd() *cobra.Command {
	var dataDir, blogDir string

	cmd := &cobra.Command{
		Use:   "blog",
		Short: "Blog consistency commands",
	}

	checkCmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check blog/data consistency",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlogCheck(dataDir, blogDir)
		},
	}
	checkCmd.Flags().StringVar(&dataDir, "data-dir", "data/gh", "data/gh path")
	checkCmd.Flags().StringVar(&blogDir, "blog-dir", "blog", "blog path")

	cmd.AddCommand(checkCmd)

	return cmd
}

func runBlogCheck(dataDir, blogDir string) error {
	result, err := workspaceuc.RunBlogCheck(workspaceuc.BlogCheckInput{
		DataDir: dataDir,
		BlogDir: blogDir,
	})
	if err != nil {
		return err
	}

	checkutil.ReportIssues(result.Issues, "blog check")
	fmt.Fprintf(os.Stderr, "summary: data/gh types=%d blog dirs=%d\n", result.GHTypes, result.BlogDirs)
	if checkutil.HasErrors(result.Issues) {
		return errors.New("blog check failed")
	}

	return nil
}
