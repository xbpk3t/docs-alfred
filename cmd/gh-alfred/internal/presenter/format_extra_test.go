package presenter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func TestFormatPlainIncludesLabels(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{
			URL:  "https://github.com/acme/tool",
			Des:  "A useful tool",
			Doc:  "data/gh/tool",
			Tag:  "kernel",
			Type: "tool",
		},
		{
			URL: "https://github.com/acme/simple",
		},
	}

	got := FormatPlain(repos, "https://docs.lucc.dev/")
	assert.Contains(t, got, "repo: https://github.com/acme/tool")
	assert.Contains(t, got, "desc: A useful tool")
	assert.Contains(t, got, "doc: data/gh/tool")
	assert.Contains(t, got, "docs: https://docs.lucc.dev/#/data/gh/tool")
	assert.Contains(t, got, "type: kernel#tool")
	assert.Contains(t, got, "repo: https://github.com/acme/simple")
}

func TestFormatPlainEmptyRepos(t *testing.T) {
	got := FormatPlain(nil, "https://docs.lucc.dev/")
	assert.Empty(t, got)
}

func TestFormatRofiIncludesFullNameAndDesc(t *testing.T) {
	repos := ghindex.Repos{
		{URL: "https://github.com/acme/tool", Des: "A tool"},
		{URL: "https://github.com/acme/simple"},
	}

	got := FormatRofi(repos)
	assert.Contains(t, got, "acme/tool - A tool")
	assert.Contains(t, got, "acme/simple")
}

func TestFormatRofiEmptyRepos(t *testing.T) {
	got := FormatRofi(nil)
	assert.Empty(t, got)
}

func TestFormatAlfredItemsNoDocNoNixNoQuery(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{URL: "https://github.com/acme/tool"},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "")
	require.Len(t, items, 1)

	assert.Equal(t, "acme/tool", items[0].Title)
	assert.NotContains(t, items[0].Mods, "cmd")
	assert.NotContains(t, items[0].Mods, "shift")
	assert.NotContains(t, items[0].Mods, "ctrl")
}

func TestFormatAlfredItemsWithNixURL(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{
			URL:    "https://github.com/acme/tool",
			NixURL: "github:acme/tool#tool",
		},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "")
	require.Len(t, items, 1)
	require.Contains(t, items[0].Mods, "ctrl")
	assert.Equal(t, "github:acme/tool#tool", items[0].Mods["ctrl"].Arg)
}

func TestFormatAlfredItemsRelatedRepo(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{
			URL:           "https://github.com/acme/tool",
			MainRepo:      "acme/main",
			IsRelatedRepo: true,
		},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "")
	require.Len(t, items, 1)
	assert.Contains(t, items[0].Subtitle, "[REL#acme/main]")
}

func TestFormatAlfredItemsTypeWithoutTag(t *testing.T) {
	useTempRepoIconCache(t)
	repos := ghindex.Repos{
		{
			URL:  "https://github.com/acme/tool",
			Type: "library",
		},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/", "")
	require.Len(t, items, 1)
	assert.Contains(t, items[0].Subtitle, "[library]")
}

func TestBuildDocURLEmptyDoc(t *testing.T) {
	assert.Empty(t, BuildDocURL("https://docs.lucc.dev/", ""))
}

func TestBuildDocURLDefaultDocsURL(t *testing.T) {
	got := BuildDocURL("", "/data/gh/foo")
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/foo", got)
}

func TestFormatRepoSubtitleNilRepo(t *testing.T) {
	assert.Empty(t, formatRepoSubtitle(nil))
}
