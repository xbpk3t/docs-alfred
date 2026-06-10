package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCommandMetadata(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "pwgen [website]", root.Use)
	require.True(t, root.HasAvailableFlags())
	require.NotNil(t, root.Flags().Lookup("secret"))
	require.NotNil(t, root.Flags().Lookup("length"))
	require.NotNil(t, root.Flags().Lookup("output"))
}

func TestRootCommandRequiresOneArg(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "arg")
}
