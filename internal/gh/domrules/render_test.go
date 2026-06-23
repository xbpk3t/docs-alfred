package domrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTopicEntries_WithTopicsAndSubs(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "overview",
				"sub": []any{
					map[string]any{"topic": "install"},
					map[string]any{"topic": "config"},
				},
			},
			map[string]any{
				"topic": "usage",
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 2)
	assert.Equal(t, "overview", topics[0].Topic)
	assert.Equal(t, []string{"install", "config"}, topics[0].Topics)
	assert.Equal(t, "usage", topics[1].Topic)
	assert.Empty(t, topics[1].Topics)
}

func TestExtractTopicEntries_NoTopics(t *testing.T) {
	entry := map[string]any{
		"type": "language",
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_EmptyTopics(t *testing.T) {
	entry := map[string]any{
		"topics": []any{},
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_NonMappingTopics(t *testing.T) {
	entry := map[string]any{
		"topics": []any{"string", 42},
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_TopicWithNilSub(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "main",
				"sub":   nil,
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 1)
	assert.Equal(t, "main", topics[0].Topic)
	assert.Nil(t, topics[0].Topics)
}

func TestExtractTopicEntries_SubWithNonMappingEntries(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "main",
				"sub":   []any{"string", 42},
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 1)
	assert.Equal(t, "main", topics[0].Topic)
	assert.Empty(t, topics[0].Topics)
}

func TestExtractTopicEntries_EmptyEntry(t *testing.T) {
	entry := map[string]any{}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopics_NonExistentFile(t *testing.T) {
	err := ExtractTopics("/tmp/nonexistent-extract-topics-output-99999.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read")
}

func TestExtractTopics_Success(t *testing.T) {
	// Create the expected file at docs/public/gh.json relative to CWD.
	// ExtractTopics uses a hardcoded relative path, so we create it at
	// the repo root which is the CWD for go test.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	repoRoot := filepath.Join(origDir, "..", "..", "..", "..")
	require.NoError(t, os.Chdir(repoRoot))

	ghJSONDir := filepath.Join(repoRoot, "docs", "public")
	require.NoError(t, os.MkdirAll(ghJSONDir, 0o755))
	ghJSONPath := filepath.Join(ghJSONDir, "gh.json")
	outPath := filepath.Join(t.TempDir(), "backbone.json")

	ghData := `[
		{
			"tag": "kernel",
			"type": "tool",
			"topics": [
				{
					"topic": "overview",
					"sub": [
						{"topic": "install"},
						{"topic": "config"}
					]
				},
				{"topic": "usage"}
			]
		},
		{
			"tag": "",
			"type": "tool",
			"topics": [{"topic": "skipped"}]
		},
		{
			"tag": "no-type",
			"type": ""
		}
	]`

	// Write gh.json and clean up after
	require.NoError(t, os.WriteFile(ghJSONPath, []byte(ghData), 0o644))
	t.Cleanup(func() { _ = os.Remove(ghJSONPath) })

	err = ExtractTopics(outPath)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "kernel")
	assert.Contains(t, string(data), "overview")
	assert.Contains(t, string(data), "install")
}

func TestExtractTopics_BadJSON(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	repoRoot := filepath.Join(origDir, "..", "..", "..", "..")
	require.NoError(t, os.Chdir(repoRoot))

	ghJSONDir := filepath.Join(repoRoot, "docs", "public")
	require.NoError(t, os.MkdirAll(ghJSONDir, 0o755))
	ghJSONPath := filepath.Join(ghJSONDir, "gh.json")
	outPath := filepath.Join(t.TempDir(), "backbone.json")

	require.NoError(t, os.WriteFile(ghJSONPath, []byte("not-json"), 0o644))
	t.Cleanup(func() { _ = os.Remove(ghJSONPath) })

	err = ExtractTopics(outPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}
