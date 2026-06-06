package cmd

import "github.com/spf13/cobra"

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check docs workspace consistency",
	}

	cmd.AddCommand(newImagesCheckCmd())
	cmd.AddCommand(newBlogCheckCmd())
	cmd.AddCommand(newDotfilesCheckCmd())

	return cmd
}
