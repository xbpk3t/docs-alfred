package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureStderr redirects os.Stderr to a pipe and returns a closer that
// restores the original and returns the captured string.
func captureStderr(t *testing.T) func() string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stderr = original
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 256)
		for {
			n, readErr := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if readErr != nil {
				break
			}
		}
		require.NoError(t, r.Close())

		return string(buf)
	}
}

// validGHYAML is a minimal valid gh.yml payload for testing.
const validGHYAML = `- type: tool
  tag: dev
  repo:
    - url: https://github.com/acme/tool
      des: "A test tool"
`

func TestExecuteRunsSuccessfully(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"gh-alfred", "--help"}

	err := Execute()
	require.NoError(t, err)
}

func TestSearchCmdRunE_ErrorNoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"search",
		"--cache", filepath.Join(t.TempDir(), "nonexistent-gh.yml"),
		"--url", srv.URL,
		"--max-age", "0s",
	})

	err := root.Execute()
	// The search error path writes Alfred JSON to stdout and returns nil.
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "Alfred index unavailable")
}

func TestSearchCmdRunE_SuccessFromCache(t *testing.T) {
	cacheDir := t.TempDir()
	cacheFile := filepath.Join(cacheDir, "gh.yml")
	require.NoError(t, os.WriteFile(cacheFile, []byte(validGHYAML), 0o600))

	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"search", "tool",
		"--cache", cacheFile,
		"--max-age", "24h",
	})

	err := root.Execute()
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "acme/tool")
}

func TestSyncCmdRunE_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	root := newRootCmd()
	root.SetArgs([]string{
		"sync",
		"--url", srv.URL,
		"--cache", filepath.Join(t.TempDir(), "out.yml"),
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "sync failed")
}

func TestSyncCmdRunE_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(validGHYAML))
	}))
	t.Cleanup(srv.Close)

	outFile := filepath.Join(t.TempDir(), "sync-out.yml")

	stderrDone := captureStderr(t)
	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"sync",
		"--url", srv.URL,
		"--cache", outFile,
	})

	err := root.Execute()
	require.NoError(t, err)

	stderrOut := stderrDone()
	require.Contains(t, stderrOut, "Syncing from")

	stdoutOut := stdout()
	require.Contains(t, stdoutOut, "Sync completed successfully")
}

func TestExportCmdRunE_Error(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"export",
		"--src", filepath.Join(t.TempDir(), "nonexistent"),
		"--out", filepath.Join(t.TempDir(), "out.yml"),
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "export gh.yml failed")
}

func TestExportCmdRunE_Success(t *testing.T) {
	src := t.TempDir()
	tagDir := filepath.Join(src, "tool")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "repo.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/tool
`), 0o644))

	outFile := filepath.Join(t.TempDir(), "out.yml")

	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"export",
		"--src", src,
		"--out", outFile,
	})

	err := root.Execute()
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "Exported "+outFile)
	require.Contains(t, got, "repos)")
}

func TestValidateCmdRunE_Error(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"validate",
		"--file", filepath.Join(t.TempDir(), "nonexistent.yml"),
	})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "validate gh.yml failed")
}

func TestValidateCmdRunE_Success(t *testing.T) {
	f := filepath.Join(t.TempDir(), "valid.yml")
	require.NoError(t, os.WriteFile(f, []byte(validGHYAML), 0o600))

	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"validate",
		"--file", f,
	})

	err := root.Execute()
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "Validated "+f)
}

func TestWriteFormatterOutput_ErrorPath(t *testing.T) {
	// Pass a channel (not JSON-marshalable) to trigger the format error path.
	err := writeFormatterOutput("alfred", make(chan int))
	require.Error(t, err)
}

func TestWriteFormatterOutput_AlfredFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeFormatterOutput("alfred", "test-item")
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "test-item")
}

func TestSearchCmdRunE_NoArgsDefaultQuery(t *testing.T) {
	cacheDir := t.TempDir()
	cacheFile := filepath.Join(cacheDir, "gh.yml")
	require.NoError(t, os.WriteFile(cacheFile, []byte(validGHYAML), 0o600))

	stdout := captureStdout(t)

	root := newRootCmd()
	root.SetArgs([]string{
		"search",
		"--cache", cacheFile,
		"--max-age", "24h",
	})

	err := root.Execute()
	require.NoError(t, err)

	got := stdout()
	require.Contains(t, got, "acme/tool")
}
