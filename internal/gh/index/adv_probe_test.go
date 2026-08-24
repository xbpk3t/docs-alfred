package ghindex

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
)

// Adversarial probe: full render→parse→ToRepos round-trip with nested rel,
// records, zk and pointer fields, asserting NO data loss and correct
// provenance reconstruction.
func TestAdv_RoundTripWithNestedRel(t *testing.T) {
	src := `- type: tool
  tag: kernel
  isDotfiles: true
  topics:
    - topic: kernel-tools
      kind: tools
      repo:
        - url: https://github.com/acme/main
          des: main repo
          nix: github:acme/main#main
          zk: kernel-main
          score: 4
          record:
            - date: "2025-01-01"
              des: init
          rel:
            - url: https://github.com/acme/rel1
              des: related one
              rel:
                - url: https://github.com/acme/rel1a
        - url: https://github.com/acme/topicrepo
          des: topic repo
`

	// 1. Render (inject tag + normalize + re-marshal) then unmarshal.
	rendered, err := NewGithubYAMLRender("kernel").Render([]byte(src))
	require.NoError(t, err)
	var cr ConfigRepos
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &cr))
	require.Len(t, cr, 1)

	// 2. ToRepos flatten.
	repos := cr.ToRepos()

	// Expect: main + rel1 + rel1a + topicrepo.
	byURL := map[string]*Repo{}
	for _, r := range repos {
		byURL[r.URL] = r
	}
	require.Len(t, repos, 4, "expected main/rel1/rel1a/topicrepo, got %d", len(repos))

	main := byURL["https://github.com/acme/main"]
	require.NotNil(t, main)
	assert.Equal(t, "kernel", main.Tag)
	assert.Equal(t, "tool", main.Type)
	require.NotNil(t, main.Des)
	assert.Equal(t, "main repo", *main.Des)
	require.NotNil(t, main.Nix)
	assert.Equal(t, "github:acme/main#main", *main.Nix)
	require.NotNil(t, main.Zk)
	assert.Equal(t, "kernel-main", *main.Zk)
	require.NotNil(t, main.Score)
	assert.Equal(t, 4, *main.Score)
	require.Len(t, main.Record, 1)
	assert.False(t, main.IsRelatedRepo)

	rel1 := byURL["https://github.com/acme/rel1"]
	require.NotNil(t, rel1)
	assert.True(t, rel1.IsRelatedRepo)
	assert.Equal(t, "acme/main", rel1.MainRepo, "rel1 MainRepo should point to main fullname")
	assert.Equal(t, "tool", rel1.Type)
	assert.Equal(t, "kernel", rel1.Tag)
	require.NotNil(t, rel1.Des)
	assert.Equal(t, "related one", *rel1.Des)

	rel1a := byURL["https://github.com/acme/rel1a"]
	require.NotNil(t, rel1a)
	assert.True(t, rel1a.IsRelatedRepo)
	// Legacy behavior: MainRepo is the immediate parent's fullname, not the
	// top-level main. Preserved from the pre-migration content.Repo code.
	assert.Equal(t, "acme/rel1", rel1a.MainRepo)

	topicRepo := byURL["https://github.com/acme/topicrepo"]
	require.NotNil(t, topicRepo)
	assert.Equal(t, "kernel-tools", topicRepo.TopicName, "topic repo gets TopicName")
	assert.Equal(t, "kernel", topicRepo.Tag)
	assert.Equal(t, "tool", topicRepo.Type)
	require.NotNil(t, topicRepo.Des)
	assert.Equal(t, "topic repo", *topicRepo.Des)
}

// Adversarial probe: the enriched Repo must marshal back to the same yaml
// keys it parses from (url/des lower-case, inline flattened, no rel loss).
func TestAdv_EnrichedRepoMarshalUnmarshal(t *testing.T) {
	des := "d"
	nix := "n"
	rec := gh.Repo{
		URL:    "https://github.com/a/b",
		Des:    &des,
		Nix:    &nix,
		Record: []gh.Record{{Date: "2025-01-01", Des: &des}},
		Rel:    []gh.Repo{{URL: "https://github.com/a/c"}},
	}
	r := &Repo{Repo: rec, Tag: "t", Type: "ty", TopicName: "tn"}

	out, err := yaml.Marshal(r)
	require.NoError(t, err)
	s := string(out)
	assert.Contains(t, s, "url: https://github.com/a/b")
	assert.NotContains(t, s, "URL:", "wire key must be lowercase url")
	assert.Contains(t, s, "tag: t")
	assert.NotContains(t, s, "TopicName", "provenance yaml:\"-\" must not serialize")

	var back Repo
	require.NoError(t, yaml.Unmarshal([]byte(s), &back))
	assert.Equal(t, "https://github.com/a/b", back.URL)
	require.NotNil(t, back.Nix)
	assert.Equal(t, "n", *back.Nix)
	require.Len(t, back.Record, 1)
	require.Len(t, back.Rel, 1)
	assert.Equal(t, "https://github.com/a/c", back.Rel[0].URL)
	assert.Equal(t, "t", back.Tag, "provenance round-trips too")
}
