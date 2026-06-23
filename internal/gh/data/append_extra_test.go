package ghdata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRecordSequence_TopicLevel(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - topic: my-topic
      record:
        - date: 2024-01-01
          des: existing
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-02-03", "new")
	require.NoError(t, err)

	updated, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "new")
}

func TestFindRecordSequence_SectionLevelFallback(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  record:
    - date: 2024-01-01
      des: existing
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// Topic not found, falls back to section-level
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "nonexistent-topic", "2024-02-03", "new")
	require.NoError(t, err)

	updated, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "new")
}

func TestFindRecordSequence_TopicNotFoundInSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  record:
    - date: 2024-01-01
      des: existing
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// Topic not found, section-level record should be used
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "missing-topic", "2024-03-01", "fallback")
	require.NoError(t, err)
}

func TestFindRecordSequence_URLNotFound(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/nonexistent", "topic", "2024-01-01", "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no section found")
}

func TestFindRecordSequence_NoRecordField(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 'record' field")
}

func TestAppendYAMLRecord_InvalidFile(t *testing.T) {
	err := appendYAMLRecord("/nonexistent/file.yml", "url", "topic", "2024-01-01", "des")
	require.Error(t, err)
}

func TestCreateRecordNode(t *testing.T) {
	node, err := createRecordNode("2024-01-01", "test record")
	require.NoError(t, err)
	require.NotNil(t, node)
}

func TestFindRecordInSequence_EmptySequence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.NoError(t, err)

	updated, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "new")
}

func TestSectionContainsURL_WithTopics(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
      topics:
        - topic: t1
          record: []
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "t1", "2024-01-01", "new")
	require.NoError(t, err)
}

func TestFindRecordSequence_TopicRecordNotSequenceFallsToSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - topic: my-topic
      record: not-a-sequence
  record:
    - date: 2024-01-01
      des: section-level
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// Topic record is not a sequence → falls back to section-level record
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-02-01", "new")
	require.NoError(t, err)
}

func TestFindRecordSequence_TopicRecordNotSequenceNoFallback(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - topic: my-topic
      record: not-a-sequence
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// Topic record not a sequence, no section-level record → error
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-01-01", "new")
	require.Error(t, err)
}

func TestFindRecordSequence_TopicNoRecordField(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - topic: my-topic
  record:
    - date: 2024-01-01
      des: existing
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// Topic found but has no record field → falls back to section-level
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-02-01", "new")
	require.NoError(t, err)
}

func TestAppendYAMLRecord_SectionRecordNotSequence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  record: not-a-sequence
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a sequence")
}

func TestFindRecordSequence_NonMappingInSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
- "just a string"
- type: dev
  repo:
    - url: https://github.com/owner/repo
  record:
    - date: 2024-01-01
      des: existing
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-02-01", "new")
	require.NoError(t, err)
}

func TestSectionContainsURL_RepoNotSequence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo: not-a-sequence
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// URL won't be found since repo is not a sequence
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.Error(t, err)
}

func TestSectionContainsURL_RepoItemNotMapping(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - "just a string"
    - url: https://github.com/owner/repo
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.NoError(t, err)
}

func TestSectionContainsURL_URLNotString(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: 12345
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "", "2024-01-01", "new")
	require.Error(t, err)
}

func TestFindTopicRecord_TopicsNotSequence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics: not-a-sequence
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// topics is not a sequence → findTopicRecord returns error → falls back to section
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "topic", "2024-01-01", "new")
	require.NoError(t, err)
}

func TestFindTopicRecord_TopicItemNotMapping(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - "just a string"
    - topic: my-topic
      record: []
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-01-01", "new")
	require.NoError(t, err)
}

func TestFindTopicRecord_TopicKeyNotString(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "repos.yml")
	content := `- type: dev
  repo:
    - url: https://github.com/owner/repo
  topics:
    - topic: 12345
  record: []
`
	require.NoError(t, os.WriteFile(file, []byte(content), 0600))

	// topic key is integer, not string → not matched → falls back to section
	err := appendYAMLRecord(file, "https://github.com/owner/repo", "my-topic", "2024-01-01", "new")
	require.NoError(t, err)
}
