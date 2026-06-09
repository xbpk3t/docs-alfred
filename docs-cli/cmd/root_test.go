package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCommandOwnsWorkspaceResources(t *testing.T) {
	root := newRootCmd()

	require.Equal(t, "docs-cli", root.Name())
	requireCommandNames(t, root.Commands(), []string{"blog", "dotfiles", "images", wikiCommandName})
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

func TestWikiCommandOwnsWikiActions(t *testing.T) {
	wikiCmd, _, err := newRootCmd().Find([]string{wikiCommandName})
	require.NoError(t, err)

	require.Equal(t, wikiCommandName, wikiCmd.Name())
	require.False(t, wikiCmd.HasAvailableFlags())
	requireCommandNames(t, wikiCmd.Commands(), []string{"add", wikiInboxCommandName, wikiAuditCommandName})
	require.Nil(t, wikiCmd.Flags().Lookup(wikiInboxCommandName))
}

func TestWikiAddCommandFlags(t *testing.T) {
	wikiAddCmd, _, err := newRootCmd().Find([]string{wikiCommandName, "add"})
	require.NoError(t, err)

	require.Equal(t, "add", wikiAddCmd.Name())
	require.True(t, wikiAddCmd.HasAvailableFlags())

	f := wikiAddCmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("wiki-root"))
	require.NotNil(t, f.Lookup("format"))
	require.NotNil(t, f.Lookup("dry-run"))
}

func TestWikiInboxCommandOwnsProcessAction(t *testing.T) {
	wikiInboxCmd, _, err := newRootCmd().Find([]string{wikiCommandName, wikiInboxCommandName})
	require.NoError(t, err)

	require.Equal(t, wikiInboxCommandName, wikiInboxCmd.Name())
	requireCommandNames(t, wikiInboxCmd.Commands(), []string{"process"})
}

func TestWikiInboxProcessCommandFlags(t *testing.T) {
	wikiInboxProcessCmd, _, err := newRootCmd().Find([]string{wikiCommandName, wikiInboxCommandName, "process"})
	require.NoError(t, err)

	require.Equal(t, "process", wikiInboxProcessCmd.Name())
	require.True(t, wikiInboxProcessCmd.HasAvailableFlags())

	f := wikiInboxProcessCmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("wiki-root"))
	require.NotNil(t, f.Lookup("format"))
	require.NotNil(t, f.Lookup("dry-run"))
}

func TestWikiAuditCommandFlags(t *testing.T) {
	wikiAuditCmd, _, err := newRootCmd().Find([]string{wikiCommandName, wikiAuditCommandName})
	require.NoError(t, err)

	require.Equal(t, wikiAuditCommandName, wikiAuditCmd.Name())
	require.True(t, wikiAuditCmd.HasAvailableFlags())

	f := wikiAuditCmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("wiki-root"))
	require.NotNil(t, f.Lookup("format"))
	require.NotNil(t, f.Lookup("changed-only"))
	require.NotNil(t, f.Lookup("paths"))
	require.Nil(t, f.Lookup("dry-run"))
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
