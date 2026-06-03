package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/blog"
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
	result, err := blog.RunCheck(dataDir, blogDir)
	if err != nil {
		return err
	}
	result.Report("blog check")
	if result.HasErrors() {
		return errors.New("blog check failed")
	}

	return nil
}
