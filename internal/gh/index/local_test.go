package ghindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSplitSection = `- type: tool
  topics:
    - topic: devops
      kind: type
  repo:
    - url: https://github.com/acme/tool
      des: Tool repository
`

func writeSplitGH(t *testing.T, root, tag, file, content string) {
	t.Helper()
	dir := filepath.Join(root, tag)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600))
}

func TestLocalTopicCatalog_FromSourceDir(t *testing.T) {
	src := t.TempDir()
	writeSplitGH(t, src, "kernel", "tool.yml", validSplitSection)

	candidates, err := LocalTopicCatalog(LocalGHConfig{SourceDir: src})
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	assert.Equal(t, "kernel/tool/devops", candidates[0].Path)
}

func TestLocalTopicCatalog_ExcludesTemp(t *testing.T) {
	src := t.TempDir()
	writeSplitGH(t, src, "kernel", "mem.yml", `- type: mem
  topics:
    - topic: futex
      kind: type
    - topic: draft
      kind: temp
`)

	candidates, err := LocalTopicCatalog(LocalGHConfig{SourceDir: src})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, "kernel/mem/futex", candidates[0].Path)
}

func TestLocalTopicCatalog_MissingSourceDir(t *testing.T) {
	_, err := LocalTopicCatalog(LocalGHConfig{
		SourceDir: filepath.Join(t.TempDir(), "nonexistent-src"),
	})
	require.Error(t, err)
}

func TestDefaultSourceDir(t *testing.T) {
	assert.Equal(t, "data/gh", DefaultSourceDir)
}

func TestSourceDirFromWikiRoot(t *testing.T) {
	assert.Equal(t, DefaultSourceDir, SourceDirFromWikiRoot(""))
	assert.Equal(t, filepath.Join("/docs", DefaultSourceDir), SourceDirFromWikiRoot("/docs/wiki"))
	assert.Equal(t, filepath.Join("/docs", DefaultSourceDir), SourceDirFromWikiRoot("/docs/wiki/"))
}

func TestLocalTopicCatalog_FromWikiRoot(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "wiki")
	src := filepath.Join(root, "data", "gh")
	require.NoError(t, os.MkdirAll(wiki, 0o750))
	writeSplitGH(t, src, "kernel", "tool.yml", validSplitSection)

	candidates, err := LocalTopicCatalog(LocalGHConfig{WikiRoot: wiki})
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	assert.Equal(t, "kernel/tool/devops", candidates[0].Path)
}
