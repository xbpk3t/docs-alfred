package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandUsesCanonicalTopLevelScopes(t *testing.T) {
	root := newRootCmd()

	requireCommandNames(t, root.Commands(), []string{"catalog", "data", "workspace"})
	requireNoCommand(t, root, "gh")
	requireNoCommand(t, root, "images")
	requireNoCommand(t, root, "blog")
	requireNoCommand(t, root, "dotfiles")
}

func TestDataCommandUsesActionFirstDomainCommands(t *testing.T) {
	dataCmd, _, err := newRootCmd().Find([]string{"data"})
	require.NoError(t, err)

	requireCommandNames(t, dataCmd.Commands(), []string{"check", "duplicate", "gh", "render"})
	requireNoCommand(t, dataCmd, "books")
	requireNoCommand(t, dataCmd, "movie")
	requireNoCommand(t, dataCmd, "music")
}

func TestWorkspaceCommandOwnsWorkspaceChecks(t *testing.T) {
	workspaceCmd, _, err := newRootCmd().Find([]string{"workspace"})
	require.NoError(t, err)

	requireCommandNames(t, workspaceCmd.Commands(), []string{"blog", "dotfiles", "images"})
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
