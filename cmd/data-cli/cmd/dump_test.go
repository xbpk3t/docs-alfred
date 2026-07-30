package cmd

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/ghcheck"
)

const multiKindGhYAML = `- type: kernel
  topics:
    - topic: futex
      kind: type
    - topic: bpf-tools
      kind: tools
    - topic: draft-notes
      kind: temp
    - topic: page-cache
      kind: mech
    - topic: linux-repos
      kind: repo
`

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = old
	out, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	require.NoError(t, r.Close())

	return string(out), runErr
}

func decodeDump(t *testing.T, raw string) []dumpTag {
	t.Helper()
	var result []dumpTag
	require.NoError(t, json.Unmarshal([]byte(raw), &result))

	return result
}

func allTopics(tags []dumpTag) []string {
	var out []string
	for _, tag := range tags {
		for _, typ := range tag.Types {
			out = append(out, typ.Topics...)
		}
	}

	return out
}

func TestParseDumpKinds(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		set, err := parseDumpKinds("")
		require.NoError(t, err)
		require.Len(t, set, len(defaultDumpKinds))
		for _, k := range defaultDumpKinds {
			_, ok := set[k]
			assert.True(t, ok, k)
		}
		_, hasTemp := set[ghcheck.KindTemp]
		assert.False(t, hasTemp)
	})

	t.Run("custom list", func(t *testing.T) {
		t.Parallel()
		set, err := parseDumpKinds(" tools , temp ")
		require.NoError(t, err)
		require.Len(t, set, 2)
		_, okTools := set["tools"]
		_, okTemp := set["temp"]
		assert.True(t, okTools)
		assert.True(t, okTemp)
	})

	t.Run("empty token error", func(t *testing.T) {
		t.Parallel()
		_, err := parseDumpKinds("tools,,temp")
		require.Error(t, err)
	})
}

func TestRunDomainDumpDefaultExcludesTemp(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"kernel/k.yml": multiKindGhYAML})

	out, err := captureStdout(t, func() error {
		return runDomainDump(data.DomainGH, ghDir, "")
	})
	require.NoError(t, err)

	topics := allTopics(decodeDump(t, out))
	assert.ElementsMatch(t, []string{"futex", "bpf-tools", "page-cache", "linux-repos"}, topics)
	assert.NotContains(t, topics, "draft-notes")
}

func TestRunDomainDumpKindsCustom(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"kernel/k.yml": multiKindGhYAML})

	out, err := captureStdout(t, func() error {
		return runDomainDump(data.DomainGH, ghDir, "tools,temp")
	})
	require.NoError(t, err)

	topics := allTopics(decodeDump(t, out))
	assert.ElementsMatch(t, []string{"bpf-tools", "draft-notes"}, topics)
}

func TestNewDumpCmdKindsFlag(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"kernel/k.yml": multiKindGhYAML})

	out, err := captureStdout(t, func() error {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"dump", "gh", "--path", ghDir, "--kinds", "type"})
		return cmd.Execute()
	})
	require.NoError(t, err)

	topics := allTopics(decodeDump(t, out))
	assert.ElementsMatch(t, []string{"futex"}, topics)
}
