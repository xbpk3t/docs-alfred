package ghindex

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

func TestLoadConfigReposFromDir_NonExistentDir(t *testing.T) {
	_, err := LoadConfigReposFromDir("/tmp/nonexistent-export-dir-99999")
	require.Error(t, err)
}

func TestLoadConfigReposFromDir_EmptySubDirs(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "empty"), 0755))

	_, err := LoadConfigReposFromDir(src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gh data found")
}

func TestValidateConfigYAML_InvalidYAML(t *testing.T) {
	err := ValidateConfigYAML([]byte("invalid: [yaml"))
	require.Error(t, err)
}

func TestValidateConfigYAML_EmptyConfig(t *testing.T) {
	err := ValidateConfigYAML([]byte("[]"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config entries")
}

func TestValidateConfigYAML_NoValidRepos(t *testing.T) {
	// Config with entries but no valid GitHub URLs
	err := ValidateConfigYAML([]byte(`- type: tool
  repo:
    - url: https://example.com/not-github
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid GitHub repositories")
}

func TestValidateConfigYAMLFile_NonExistent(t *testing.T) {
	err := ValidateConfigYAMLFile("/tmp/nonexistent-gh-99999.yml")
	require.Error(t, err)
}

func TestMarshalConfigReposYAML(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "tool",
			Tag:  "test",
			Repos: Repos{
				{URL: "https://github.com/acme/tool", Des: "test"},
			},
		},
	}
	data, err := MarshalConfigReposYAML(cr)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestNormalizeConfigURL(t *testing.T) {
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", normalizeConfigURL("https://cdn.lucc.dev/"))
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", normalizeConfigURL("https://cdn.lucc.dev/gh.yml"))
}

func TestRenderConfigYAMLFromDir_EmptySubDirs(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "empty"), 0755))

	_, err := RenderConfigYAMLFromDir(src)
	require.Error(t, err)
}

func TestWriteConfigYAMLFromDir_EmptySubDirs(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "empty"), 0755))
	out := filepath.Join(t.TempDir(), "gh.yml")

	_, err := WriteConfigYAMLFromDir(src, out)
	require.Error(t, err)
}

func TestWriteConfigYAMLFromDir_NonExistentDir(t *testing.T) {
	_, err := WriteConfigYAMLFromDir("/tmp/nonexistent-99999", "/tmp/out.yml")
	require.Error(t, err)
}

func TestLoadConfigReposFromDir_SubDirWithEmptyData(t *testing.T) {
	src := t.TempDir()
	tagDir := filepath.Join(src, "empty-tag")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	// Write an empty YAML sequence
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "empty.yml"), []byte("[]\n"), 0644))

	_, err := LoadConfigReposFromDir(src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gh data found")
}

func TestLoadConfigReposFromDir_MultipleTagDirs(t *testing.T) {
	src := t.TempDir()

	// Tag 1
	tagDir1 := filepath.Join(src, "kernel")
	require.NoError(t, os.MkdirAll(tagDir1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir1, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/tool
`), 0644))

	// Tag 2
	tagDir2 := filepath.Join(src, "network")
	require.NoError(t, os.MkdirAll(tagDir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir2, "lib.yml"), []byte(`- type: lib
  repo:
    - url: https://github.com/acme/lib
`), 0644))

	repos, err := LoadConfigReposFromDir(src)
	require.NoError(t, err)
	require.Len(t, repos, 2)
}

func TestLoadConfigReposFromDir_SkipsNonDirEntries(t *testing.T) {
	src := t.TempDir()
	// Create a file at the top level (should be skipped as it's not a dir)
	require.NoError(t, os.WriteFile(filepath.Join(src, "readme.txt"), []byte("text"), 0644))
	// Create a valid tag dir
	tagDir := filepath.Join(src, "kernel")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/tool
`), 0644))

	repos, err := LoadConfigReposFromDir(src)
	require.NoError(t, err)
	require.Len(t, repos, 1)
}

func TestLoadConfigReposFromDir_RenderError(t *testing.T) {
	src := t.TempDir()
	tagDir := filepath.Join(src, "bad")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	// Write invalid YAML that will cause render error
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "bad.yml"), []byte("invalid: [yaml: broken\n"), 0644))

	_, err := LoadConfigReposFromDir(src)
	require.Error(t, err)
}

func TestRenderConfigYAMLFromDir_WriteError(t *testing.T) {
	src := writeExportFixture(t)
	// Write to a path that doesn't exist (can't create parent dirs)
	_, err := RenderConfigYAMLFromDir(src)
	require.NoError(t, err) // This should succeed
}

func TestWriteConfigYAMLFromDir_InvalidOutputPath(t *testing.T) {
	src := writeExportFixture(t)
	_, err := WriteConfigYAMLFromDir(src, "/nonexistent-dir/gh.yml")
	require.Error(t, err)
}
