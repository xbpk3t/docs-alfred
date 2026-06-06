package gh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDocURL(t *testing.T) {
	assert.Equal(t, "https://example.com/doc", BuildDocURL("https://docs.lucc.dev", "https://example.com/doc"))
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/foo", BuildDocURL("https://docs.lucc.dev/", "/data/gh/foo"))
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/foo", BuildDocURL("https://docs.lucc.dev/#/data/gh", "data/gh/foo"))
}

func TestFormatAlfredItemsBuildsRepoAndDocActions(t *testing.T) {
	repos := Repos{
		{
			URL: "https://github.com/acme/tool",
			Des: "Tooling",
			Doc: "data/gh/tool",
			Tag: "kernel",
		},
		{
			URL: "https://github.com/acme/external-doc",
			Doc: "https://example.com/external-doc",
		},
	}

	items := FormatAlfredItems(repos, "https://docs.lucc.dev/")
	require.Len(t, items, 2)

	assert.Equal(t, "acme/tool", items[0].Title)
	assert.Equal(t, "https://github.com/acme/tool", items[0].Arg)
	assert.Equal(t, "acme/tool", items[0].Autocomplete)
	require.NotNil(t, items[0].Text)
	assert.Equal(t, "https://github.com/acme/tool", items[0].Text.Copy)
	require.Contains(t, items[0].Mods, "cmd")
	assert.Equal(t, "https://docs.lucc.dev/#/data/gh/tool", items[0].Mods["cmd"].Arg)

	require.Contains(t, items[1].Mods, "cmd")
	assert.Equal(t, "https://example.com/external-doc", items[1].Mods["cmd"].Arg)
}
