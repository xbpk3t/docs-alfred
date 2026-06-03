package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newBlogCmd creates `blog check`.
func newBlogCmd() *cobra.Command {
	var dataDir, blogDir string

	cmd := &cobra.Command{
		Use:   "blog",
		Short: "Blog consistency commands",
	}

	checkCmd := &cobra.Command{
		Use:   "check",
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
	fmt.Fprintf(os.Stderr, "Checking blog consistency: data-dir=%q blog-dir=%q...\n", dataDir, blogDir)
	// TODO: full port from TS modules/blog/check.ts
	return nil
}
