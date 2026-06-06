package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsWorkspaceActions(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "docs-cli", root.Name())
	requireCommandNames(t, root.Commands(), []string{"check", "sync-plan"})
	requireNoCommand(t, root, "alfred")
	requireNoCommand(t, root, "data")
	requireNoCommand(t, root, "workspace")
}

func TestCheckCommandOwnsWorkspaceChecks(t *testing.T) {
	checkCmd, _, err := newRootCmd().Find([]string{"check"})
	require.NoError(t, err)

	requireCommandNames(t, checkCmd.Commands(), []string{"blog", "dotfiles", "images"})
}

func TestSyncPlanCommandOwnsWorkspaceSyncPlans(t *testing.T) {
	syncPlanCmd, _, err := newRootCmd().Find([]string{"sync-plan"})
	require.NoError(t, err)

	requireCommandNames(t, syncPlanCmd.Commands(), []string{"dotfiles"})
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
