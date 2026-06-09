package datarender

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

func TestProcessRenderConfigEmpty(t *testing.T) {
	cfg := processRenderConfig(docsConfig{})
	assert.Equal(t, "", cfg.Src)
}

func TestProcessRenderConfigWithJSON(t *testing.T) {
	proc := newDocProcessor(fileTypeJSON)
	proc.Dst = "output.json"

	cfg := processRenderConfig(docsConfig{
		Src:  "test",
		JSON: proc,
	})

	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.JSON)
	assert.Equal(t, "output.json", cfg.JSON.Dst)
}

func TestProcessRenderConfigWithYAML(t *testing.T) {
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = "output.yml"

	cfg := processRenderConfig(docsConfig{
		Src:  "test",
		YAML: proc,
	})

	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.YAML)
	assert.Equal(t, "output.yml", cfg.YAML.Dst)
}

func TestProcessRenderConfigsReturnsProcessError(t *testing.T) {
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = t.TempDir()

	err := processRenderConfigs([]docsConfig{{
		Src:  "missing.yml",
		Cmd:  "gh",
		YAML: proc,
	}})

	require.Error(t, err)
}

func TestRunPreservesGithubNixMetadataInYAMLAndJSONOutputs(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "kernel")
	outDir := filepath.Join(tmpDir, "public")
	require.NoError(t, os.MkdirAll(tagDir, 0755))

	mainNix := "https://mynixos.com/nixpkgs/package/main-tool"
	relatedNix := "https://mynixos.com/nixpkgs/package/related-tool"
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
      des: Main tool
      doc: data/gh/main-tool
      nix: https://mynixos.com/nixpkgs/package/main-tool
      rel:
        - url: https://github.com/acme/related-tool
          des: Related tool
          nix: https://mynixos.com/nixpkgs/package/related-tool
`), 0644))

	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  src,
		Cmd:  "gh",
		JSON: &docProcessor{Dst: outDir},
		YAML: &docProcessor{Dst: outDir},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	yamlData, err := os.ReadFile(filepath.Join(outDir, "gh.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "nix: "+mainNix)
	assert.Contains(t, string(yamlData), "nix: "+relatedNix)

	var configRepos ghindex.ConfigRepos
	require.NoError(t, yaml.Unmarshal(yamlData, &configRepos))
	require.Len(t, configRepos, 1)
	require.Len(t, configRepos[0].Repos, 1)
	mainRepo := configRepos[0].Repos[0]
	assert.Equal(t, mainNix, mainRepo.NixURL)
	require.Len(t, mainRepo.RelatedRepos, 1)
	assert.Equal(t, relatedNix, mainRepo.RelatedRepos[0].NixURL)

	jsonData, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	var jsonRepos []map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &jsonRepos))
	require.Len(t, jsonRepos, 1)
	jsonRepo, ok := jsonRepos[0]["repo"].([]any)
	require.True(t, ok)
	require.Len(t, jsonRepo, 1)
	jsonMainRepo, ok := jsonRepo[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, mainNix, jsonMainRepo["nix"])
	jsonRelatedRepos, ok := jsonMainRepo["rel"].([]any)
	require.True(t, ok)
	require.Len(t, jsonRelatedRepos, 1)
	jsonRelatedRepo, ok := jsonRelatedRepos[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, relatedNix, jsonRelatedRepo["nix"])
}
