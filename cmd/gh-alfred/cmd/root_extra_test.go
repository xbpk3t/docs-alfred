package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func TestSearchCmdFlags(t *testing.T) {
	searchCmd, _, err := newRootCmd().Find([]string{"search"})
	require.NoError(t, err)

	f := searchCmd.Flags()
	require.NotNil(t, f.Lookup("url"))
	require.NotNil(t, f.Lookup("cache"))
	require.NotNil(t, f.Lookup("docs-url"))
	require.NotNil(t, f.Lookup("max-age"))
}

func TestSyncCmdFlags(t *testing.T) {
	syncCmd, _, err := newRootCmd().Find([]string{"sync"})
	require.NoError(t, err)

	f := syncCmd.Flags()
	require.NotNil(t, f.Lookup("url"))
	require.NotNil(t, f.Lookup("cache"))
}

func TestExportCmdFlags(t *testing.T) {
	exportCmd, _, err := newRootCmd().Find([]string{"export"})
	require.NoError(t, err)

	f := exportCmd.Flags()
	require.NotNil(t, f.Lookup("src"))
	require.NotNil(t, f.Lookup("out"))
}

func TestValidateCmdFlags(t *testing.T) {
	validateCmd, _, err := newRootCmd().Find([]string{"validate"})
	require.NoError(t, err)

	f := validateCmd.Flags()
	require.NotNil(t, f.Lookup("file"))
}

func captureStdout(t *testing.T) func() string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stdout = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		return string(data)
	}
}

func TestWriteOutputWritesToStdout(t *testing.T) {
	stdout := captureStdout(t)

	err := writeOutput("hello world")
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "hello world\n")
}

func TestRunSearchOutputWritesAlfredJSON(t *testing.T) {
	stdout := captureStdout(t)

	repos := ghindex.Repos{
		{URL: "https://github.com/acme/tool", Des: "A tool"},
	}

	err := runSearchOutput(repos, "https://docs.lucc.dev/", "tool")
	require.NoError(t, err)

	got := stdout()
	require.NotEmpty(t, got)
	require.Contains(t, got, "acme/tool")
}
