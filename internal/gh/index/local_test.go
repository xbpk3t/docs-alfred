package ghindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validGHYML = `- type: tool
  tag: kernel
  repo:
    - url: https://github.com/acme/tool
      des: Tool repository
      topics:
        - topic: devops
`

func TestLoadLocalGHYML_FileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gh.yml")
	require.NoError(t, os.WriteFile(path, []byte(validGHYML), 0o600))

	configRepos, err := LoadLocalGHYML(LocalGHConfig{Path: path})
	require.NoError(t, err)
	require.Len(t, configRepos, 1)
	assert.Equal(t, "kernel", configRepos[0].Tag)
}

func TestLoadLocalGHYML_LazyGenerate(t *testing.T) {
	srcDir := t.TempDir()
	tagDir := filepath.Join(srcDir, "kernel")
	require.NoError(t, os.MkdirAll(tagDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: Tool repository
`), 0o600))

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "sub", "gh.yml")

	configRepos, err := LoadLocalGHYML(LocalGHConfig{Path: outPath, SourceDir: srcDir})
	require.NoError(t, err)
	require.Len(t, configRepos, 1)
	assert.Equal(t, "kernel", configRepos[0].Tag)

	// Verify file was written
	_, err = os.Stat(outPath)
	require.NoError(t, err)
}

func TestLoadLocalGHYML_NoFileNoSourceDir(t *testing.T) {
	_, err := LoadLocalGHYML(LocalGHConfig{
		Path:      filepath.Join(t.TempDir(), "nonexistent", "gh.yml"),
		SourceDir: filepath.Join(t.TempDir(), "nonexistent-src"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run `task data` to generate")
}

func TestLoadLocalGHYML_DefaultPath(t *testing.T) {
	// Verify default path constant is set
	assert.Equal(t, "/tmp/gh.yml", LocalGHYMLPath)
}

func TestLocalTopicCatalog_FileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gh.yml")
	require.NoError(t, os.WriteFile(path, []byte(validGHYML), 0o600))

	candidates, err := LocalTopicCatalog(LocalGHConfig{Path: path})
	require.NoError(t, err)
	assert.NotEmpty(t, candidates)
}

func TestLocalTopicCatalog_NoFile(t *testing.T) {
	_, err := LocalTopicCatalog(LocalGHConfig{
		Path:      filepath.Join(t.TempDir(), "nonexistent", "gh.yml"),
		SourceDir: filepath.Join(t.TempDir(), "nonexistent-src"),
	})
	require.Error(t, err)
}
