package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	docscli "github.com/xbpk3t/docs-alfred/docs-cli/pkg"
)

func TestProcessRenderConfig_Empty(t *testing.T) {
	cfg := processRenderConfig(docscli.DocsConfig{})
	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.Src)
}

func TestProcessRenderConfig_WithJSON(t *testing.T) {
	proc := docscli.NewDocProcessor(docscli.FileTypeJSON)
	proc.Dst = "output.json"
	cfg := processRenderConfig(docscli.DocsConfig{
		Src:  "test",
		JSON: proc,
	})
	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.JSON)
	assert.Equal(t, "output.json", cfg.JSON.Dst)
}

func TestProcessRenderConfig_WithYAML(t *testing.T) {
	proc := docscli.NewDocProcessor(docscli.FileTypeYAML)
	proc.Dst = "output.yml"
	cfg := processRenderConfig(docscli.DocsConfig{
		Src:  "test",
		YAML: proc,
	})
	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.YAML)
	assert.Equal(t, "output.yml", cfg.YAML.Dst)
}
