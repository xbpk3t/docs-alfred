package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsAlfredActions(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "gh-alfred", root.Name())
	requireCommandNames(t, root.Commands(), []string{"export", "schema", "search", "sync", "validate"})
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
