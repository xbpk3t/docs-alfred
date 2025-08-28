// Package cmd provides CLI commands for the Alfred GitHub workflow
package cmd

import (
	"github.com/spf13/cobra"
)

// GithubParam represents a parameter type for GitHub operations
type GithubParam string

const (
	// ParamRepo represents repository parameter
	ParamRepo GithubParam = "repo"
	// ParamTag represents tag parameter
	ParamTag GithubParam = "tag"
	// ParamType represents type parameter
	ParamType GithubParam = "type"
)

// createGhCmd creates the gh subcommand
func createGhCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gh",
		Short: "Searching from starred repositories and my repositories",
		Run: func(_ *cobra.Command, _ []string) {
			// TODO: Refactor this command to work with new structure
		},
	}
}
