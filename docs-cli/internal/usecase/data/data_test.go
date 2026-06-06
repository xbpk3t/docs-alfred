package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
