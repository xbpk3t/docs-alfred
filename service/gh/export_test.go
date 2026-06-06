package gh

import (
	"os"
	"path/filepath"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderConfigYAMLFromDirBuildsValidatedRemoteArtifact(t *testing.T) {
	src := writeExportFixture(t)

	data, err := RenderConfigYAMLFromDir(src)
	require.NoError(t, err)
	require.NoError(t, ValidateConfigYAML(data))

	var configRepos ConfigRepos
	require.NoError(t, yaml.Unmarshal(data, &configRepos))
	require.Len(t, configRepos, 1)
	assert.Equal(t, "kernel", configRepos[0].Tag)
	assert.Equal(t, "tool", configRepos[0].Type)
	require.Len(t, configRepos[0].Repos, 1)
	assert.Equal(t, "https://github.com/acme/tool", configRepos[0].Repos[0].URL)
}

func TestWriteConfigYAMLFromDirWritesValidatedRemoteArtifact(t *testing.T) {
	src := writeExportFixture(t)
	out := filepath.Join(t.TempDir(), "nested", "gh.yml")

	repoCount, err := WriteConfigYAMLFromDir(src, out)
	require.NoError(t, err)
	assert.Equal(t, 1, repoCount)
	require.NoError(t, ValidateConfigYAMLFile(out))
}

func writeExportFixture(t *testing.T) string {
	t.Helper()

	src := t.TempDir()
	tagDir := filepath.Join(src, "kernel")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: Tool repository
      doc: data/gh/tool
`), 0644))

	return src
}
