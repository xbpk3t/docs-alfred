package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsDataActions(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "data-cli", root.Name())
	requireCommandNames(t, root.Commands(), []string{"check", "duplicate", "gh", "render"})
}

func TestGhCommandOwnsDataGhActions(t *testing.T) {
	ghCmd, _, err := newRootCmd().Find([]string{"gh"})
	require.NoError(t, err)

	requireCommandNames(t, ghCmd.Commands(), []string{"append-record", "find"})
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
