package presenter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func useTempRepoIconCache(t *testing.T) string {
	t.Helper()

	old := repoIconCacheDir
	dir := t.TempDir()
	repoIconCacheDir = dir
	t.Cleanup(func() {
		repoIconCacheDir = old
	})

	return dir
}

func TestBuildDocURL(t *testing.T) {
	assert.Equal(t, "https://example.com/doc", BuildDocURL("https://docs.lucc.dev", "https://example.com/doc"))
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/foo", BuildDocURL("https://docs.lucc.dev/", "/data/gh/foo"))
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/foo", BuildDocURL("https://docs.lucc.dev/#/data/gh", "data/gh/foo"))
}

func TestFormatAlfredItemsBuildsRepoAndDocActions(t *testing.T) {
	cacheDir := useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{
			URL:      "https://github.com/acme/tool",
			Des:      "Tooling",
			Doc:      "data/gh/tool",
			Tag:      "kernel",
			Type:     "tool",
			Topics:   content.Topics{{Topic: "install"}},
			MainRepo: "acme/main",
		},
		{
			URL: "https://github.com/acme/external-doc",
			Doc: "https://example.com/external-doc",
		},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "")
	require.Len(t, items, 2)

	assert.Equal(t, "acme/tool", items[0].Title)
	assert.Equal(t, "https://github.com/acme/tool", items[0].Arg)
	assert.Equal(t, "acme/tool", items[0].Autocomplete)
	assert.Equal(t, "[kernel#tool] Tooling", items[0].Subtitle)
	assert.Equal(t, filepath.Join(cacheDir, "gh-d1-n0-s0.svg"), items[0].Icon.Path)
	require.NotNil(t, items[0].Text)
	assert.Equal(t, "https://github.com/acme/tool", items[0].Text.Copy)
	require.Contains(t, items[0].Mods, "alt")
	assert.Equal(t, "https://github.com/acme/tool", items[0].Mods["alt"].Arg)
	require.Contains(t, items[0].Mods, "cmd")
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/tool", items[0].Mods["cmd"].Arg)
	require.Contains(t, items[0].Mods, "shift")
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/tool", items[0].Mods["shift"].Arg)
	assert.Equal(t, "打开文档: https://docs.lucc.dev/#/data/gh/tool", items[0].Mods["shift"].Subtitle)

	require.Contains(t, items[1].Mods, "alt")
	assert.Equal(t, "https://github.com/acme/external-doc", items[1].Mods["alt"].Arg)
	require.Contains(t, items[1].Mods, "cmd")
	assert.Equal(t, "https://example.com/external-doc", items[1].Mods["cmd"].Arg)
}

func TestFormatAlfredItemsAddsGitHubSearchFallbackForQueries(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{{URL: "https://github.com/acme/tool"}}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "tool kit")
	require.Len(t, items, 2)

	assert.Equal(t, "Search GitHub: tool kit", items[1].Title)
	assert.Equal(t, "https://github.com/search?q=tool+kit&type=repositories", items[1].Arg)
}
