package ghindex

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubYAMLRenderParsesTopics(t *testing.T) {
	input := []byte(`
---
- type: HTTP
  topics:
    - topic: websocket
    - topic: explicit
`)

	rendered, err := NewGithubYAMLRender("kernel").Render(input)
	require.NoError(t, err)

	var repos ConfigRepos
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &repos))
	require.Len(t, repos, 1)

	cfg := repos[0]
	assert.Equal(t, "kernel", cfg.Tag)

	require.Len(t, cfg.Topics, 2)
	assert.Equal(t, "websocket", cfg.Topics[0].Topic)
	assert.Equal(t, "explicit", cfg.Topics[1].Topic)
}

func TestGithubYAMLRender_InvalidInput(t *testing.T) {
	r := NewGithubYAMLRender("test")
	_, err := r.Render([]byte("invalid: [yaml: broken"))
	require.Error(t, err)
}

func TestGithubYAMLRender_EmptyInput(t *testing.T) {
	r := NewGithubYAMLRender("test")
	result, err := r.Render([]byte("[]"))
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGithubYAMLRender_TagAlreadySet(t *testing.T) {
	input := []byte(`---
- type: tool
  tag: custom-tag
`)
	r := NewGithubYAMLRender("default-tag")
	rendered, err := r.Render(input)
	require.NoError(t, err)
	assert.Contains(t, rendered, "custom-tag")
}

func TestGithubYAMLRender_GetCurrentFileName(t *testing.T) {
	r := NewGithubYAMLRender("test")
	assert.Empty(t, r.GetCurrentFileName())
}
