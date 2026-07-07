package ghindex

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubYAMLRenderNormalizesTopicDisplayFields(t *testing.T) {
	input := []byte(`
---
- type: HTTP
  topics:
    - topic: websocket
    - topic: explicit
      picDir: custom/path
  repo:
    - url: https://github.com/acme/main-repo.git
      nix: github:acme/main-repo#main-repo
      topics:
        - topic: Parent Topic
          meta:
            slug: parent-topic
          sub:
            - topic: Child Topic
              meta:
                slug: child-topic
                isX: true
`)

	rendered, err := NewGithubYAMLRender("kernel").Render(input)
	require.NoError(t, err)

	var repos ConfigRepos
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &repos))
	require.Len(t, repos, 1)

	cfg := repos[0]
	assert.Equal(t, "kernel", cfg.Tag)

	require.Len(t, cfg.Topics, 2)
	assert.True(t, cfg.Topics[0].PicDir != "")
	assert.Equal(t, "kernel/HTTP/websocket", cfg.Topics[0].PicDir)
	assert.Nil(t, cfg.Topics[0].Meta)
	assert.Equal(t, "custom/path", cfg.Topics[1].PicDir)

	require.Len(t, cfg.Repos, 1)
	assert.Equal(t, "github:acme/main-repo#main-repo", cfg.Repos[0].NixURL)
	require.Len(t, cfg.Repos[0].Topics, 1)
	parent := cfg.Repos[0].Topics[0]
	assert.True(t, parent.PicDir != "")
	assert.Equal(t, "kernel/HTTP/main-repo/parent-topic", parent.PicDir)
	assert.Nil(t, parent.Meta)

	require.Len(t, parent.Sub, 1)
	child := parent.Sub[0]
	assert.True(t, child.PicDir != "")
	assert.True(t, child.IsX)
	assert.Equal(t, "kernel/HTTP/main-repo/parent-topic/child-topic", child.PicDir)
	assert.Nil(t, child.Meta)
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
  repo:
    - url: https://github.com/acme/repo
`)
	r := NewGithubYAMLRender("default-tag")
	rendered, err := r.Render(input)
	require.NoError(t, err)
	assert.Contains(t, rendered, "custom-tag")
}

func TestNormalizeRepoTopics_NilRepo(t *testing.T) {
	// Should not panic
	normalizeRepoTopics(nil, "base", false)
}

func TestNormalizeRepoTopics_EmptyURL(t *testing.T) {
	repo := &Repository{
		URL: "",
	}
	normalizeRepoTopics(repo, "base", false)
	// Should not panic; empty repo name means return early
}

func TestNormalizeRepoTopics_UseBase(t *testing.T) {
	repo := &Repository{
		URL: "https://github.com/acme/repo",
	}
	normalizeRepoTopics(repo, "base", true)
	// Should not panic
}

func TestGithubYAMLRender_GetCurrentFileName(t *testing.T) {
	r := NewGithubYAMLRender("test")
	assert.Empty(t, r.GetCurrentFileName())
}
