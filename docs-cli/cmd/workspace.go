package cmd

import "github.com/spf13/cobra"

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Workspace consistency and asset commands",
	}

	cmd.AddCommand(newImagesCmd())
	cmd.AddCommand(newDotfilesCmd())
	cmd.AddCommand(newBlogCmd())

	return cmd
}
