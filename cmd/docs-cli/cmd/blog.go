package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

func newBlogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blog",
		Short: "Blog knowledge operations (sibling of wiki)",
	}

	cmd.AddCommand(newBlogCheckCmd())

	return cmd
}

func newBlogCheckCmd() *cobra.Command {
	var flags struct {
		blogRoot string
	}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check blog/ OKF frontmatter compliance",
		Long:  `Validate OKF v0.1 frontmatter on all blog posts (blog/<tag>/<type>/<file>.md).`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			issues, err := workspaceuc.RunBlogCheckOKF(flags.blogRoot)
			if err != nil {
				return err
			}
			textDetails := fmt.Sprintf("summary: issues=%d\n", len(issues))
			if err := writeCheckCommandOutput(output.GetFormat(cmd), &checkCommandOutput{
				Name:    "blog check",
				Issues:  issues,
				Summary: map[string]any{"issues": len(issues)},
			}, textDetails); err != nil {
				return err
			}
			if checkutil.HasErrors(issues) {
				return errors.New("blog check failed")
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flags.blogRoot, "blog-root", "blog", "blog path")

	return cmd
}
