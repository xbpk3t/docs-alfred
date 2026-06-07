package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsWorkspaceResources(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "docs-cli", root.Name())
	requireCommandNames(t, root.Commands(), []string{"blog", "dotfiles", "images"})
	requireNoCommand(t, root, cmdCheck)
	requireNoCommand(t, root, "sync-record")
	requireNoCommand(t, root, "alfred")
	requireNoCommand(t, root, "data")
	requireNoCommand(t, root, "workspace")
}

func TestImagesCommandOwnsImageActions(t *testing.T) {
	imagesCmd, _, err := newRootCmd().Find([]string{"images"})
	require.NoError(t, err)

	requireCommandNames(t, imagesCmd.Commands(), []string{cmdCheck, "fix"})
}

func TestBlogCommandOwnsBlogActions(t *testing.T) {
	blogCmd, _, err := newRootCmd().Find([]string{"blog"})
	require.NoError(t, err)

	requireCommandNames(t, blogCmd.Commands(), []string{cmdCheck})
}

func TestDotfilesCommandOwnsDotfilesActions(t *testing.T) {
	dotfilesCmd, _, err := newRootCmd().Find([]string{"dotfiles"})
	require.NoError(t, err)

	requireCommandNames(t, dotfilesCmd.Commands(), []string{cmdCheck, "sync-record"})
}

func requireCommandNames(t *testing.T, commands []*cobra.Command, want []string) {
	t.Helper()

	got := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if cmd.Hidden {
			continue
		}
		got = append(got, cmd.Name())
	}

	require.ElementsMatch(t, want, got)
}

func requireNoCommand(t *testing.T, parent *cobra.Command, name string) {
	t.Helper()

	for _, cmd := range parent.Commands() {
		require.NotEqual(t, name, cmd.Name())
	}
}
