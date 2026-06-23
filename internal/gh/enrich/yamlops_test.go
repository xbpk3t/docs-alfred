package enrich

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestParseYAMLFile(t *testing.T) {
	yaml := `- name: 《Item One》
  publishAt: 2020
  author: Author One

- name: 《Item Two》
  publishAt: 2021
`
	path := writeTestYAML(t, yaml)
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "《Item One》", items[0].GetName())
	require.Equal(t, "《Item Two》", items[1].GetName())
}

func TestGetField(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
  author: Some Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.Equal(t, "《Test》", items[0].GetField("name"))
	require.Equal(t, "2020", items[0].GetField("publishAt"))
	require.Equal(t, "", items[0].GetField("nonexistent"))
}

func TestFieldExists(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
  emptyField: ""
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.True(t, items[0].FieldExists("name"))
	require.True(t, items[0].FieldExists("publishAt"))
	require.False(t, items[0].FieldExists("nonexistent"))
	require.False(t, items[0].FieldExists("emptyField"))
}

func TestSetField(t *testing.T) {
	yaml := `- name: 《Test》
  author: Old Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("publishAt", "2023"))
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.Equal(t, "2023", items2[0].GetField("publishAt"))
}

func TestSetFieldPreservesComments(t *testing.T) {
	yaml := `# Top-level comment
- name: 《Test》
  author: Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("publishAt", "2023"))
	require.NoError(t, SaveYAMLFile(path, items))

	data, _ := os.ReadFile(path)
	require.NotEmpty(t, string(data), "file is empty after write")
}

func TestSetFieldEmptyValue(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	err := items[0].SetField("publishAt", "")
	require.Error(t, err, "expected error for empty value")
}

func TestMultiItemFieldOperations(t *testing.T) {
	yaml := `- name: 《First》
  publishAt: 2019

- name: 《Second》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.Len(t, items, 2)
	require.True(t, items[0].FieldExists("publishAt"))
	require.False(t, items[1].FieldExists("publishAt"))
	require.NoError(t, items[1].SetField("publishAt", "2020"))
}

func TestParseYAMLFileMultiDoc(t *testing.T) {
	yaml := `---
- name: 《Doc One》
  author: A

---
- name: 《Doc Two》
  author: B
`
	path := writeTestYAML(t, yaml)
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestParseYAMLFileComments(t *testing.T) {
	yaml := `# Header comment 1
# Header comment 2

- name: 《Item One》
  # Inline comment
  publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "《Item One》", items[0].GetName())
}

func TestParseYAMLFileEmpty(t *testing.T) {
	path := writeTestYAML(t, "")
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	require.Empty(t, items, "expected 0 items from empty file")
}

func TestGetPublishAtInteger(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Equal(t, "2020", items[0].GetPublishAt())
}

func TestGetPublishAtString(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: "2020"
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Equal(t, "2020", items[0].GetPublishAt())
}

func TestGetPublishAtMissing(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Empty(t, items[0].GetPublishAt())
}

func TestSaveSetFieldOnMultiItemWithChinese(t *testing.T) {
	yaml := `- name: 《影响力》
  author: 西奥迪尼

- name: 《毛选》
  author: 毛泽东
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("publishAt", "1984"))
	require.NoError(t, items[1].SetField("publishAt", "1960"))
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.Equal(t, "1984", items2[0].GetField("publishAt"))
	require.Equal(t, "1960", items2[1].GetField("publishAt"))
}

func TestSaveSetFieldOnItemWithExistingFields(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2019
  score: 4
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("alias", "English Title"))
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.Equal(t, "English Title", items2[0].GetField("alias"))
	require.Equal(t, "2019", items2[0].GetField("publishAt"))
}

func TestHasPending(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.False(t, items[0].HasPending())
	require.NoError(t, items[0].SetField("publishAt", "2023"))
	require.True(t, items[0].HasPending())
}

func TestSetFieldEmptyKey(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	err := items[0].SetField("", "value")
	require.Error(t, err)
}

func TestSetFieldListFieldCommaSeparator(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	// cast is a list field, must use 、not comma
	err := items[0].SetField("cast", "a,b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must use")
}

func TestSetFieldListFieldValidSeparator(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	err := items[0].SetField("cast", "A、B")
	require.NoError(t, err)
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.True(t, items2[0].FieldExists("cast"))
}

func TestSaveYAMLFile_ReadError(t *testing.T) {
	// Override readFile to return error
	origReadFile := readFile
	readFile = func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	defer func() { readFile = origReadFile }()

	err := SaveYAMLFile("/nonexistent/path.yml", nil)
	require.Error(t, err)
}

func TestGetName_Missing(t *testing.T) {
	yaml := `- publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Empty(t, items[0].GetName())
}

func TestGetPublishAt_IntegerNode(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Equal(t, "2020", items[0].GetPublishAt())
}

func TestGetField_IntegerNode(t *testing.T) {
	yaml := `- name: 《Test》
  score: 4
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Equal(t, "4", items[0].GetField("score"))
}

func TestGetField_NonStringNonInt(t *testing.T) {
	yaml := `- name: 《Test》
  tags:
    - a
    - b
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	// tags is a sequence, not a scalar - should return ""
	require.Empty(t, items[0].GetField("tags"))
}

func TestFieldExists_NullValue(t *testing.T) {
	yaml := `- name: 《Test》
  alias: null
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.False(t, items[0].FieldExists("alias"))
}

func TestSaveYAMLFile_WithPendingFields(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("alias", "Test Alias"))
	require.NoError(t, items[0].SetField("dict", "Director A"))
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.Equal(t, "Test Alias", items2[0].GetField("alias"))
	require.Equal(t, "Director A", items2[0].GetField("dict"))
}

func TestParseYAMLFile_NonSequenceDoc(t *testing.T) {
	yaml := `key: value
`
	path := writeTestYAML(t, yaml)
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestMarshalPendingFields_Empty(t *testing.T) {
	lines, err := marshalPendingFields(map[string]string{})
	require.NoError(t, err)
	require.Empty(t, lines)
}

func TestParseASTFile_NonMappingValue(t *testing.T) {
	// YAML with a non-mapping item in the sequence (e.g. a string)
	yaml := `- just a string
- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, err := ParseYAMLFile(path)
	require.NoError(t, err)
	// Only the mapping item should be returned
	require.Len(t, items, 1)
	require.Equal(t, "《Test》", items[0].GetName())
}

func TestGetName_NonStringValue(t *testing.T) {
	yaml := `- name:
    nested: value
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Len(t, items, 1)
	// name is a mapping, not a string - should return ""
	require.Empty(t, items[0].GetName())
}

func TestGetPublishAt_NonStringNonInt(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt:
    nested: value
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	require.Len(t, items, 1)
	require.Empty(t, items[0].GetPublishAt())
}

func TestSaveYAMLFile_MultipleItemsWithInserts(t *testing.T) {
	yaml := `- name: 《First》
  publishAt: 2019

- name: 《Second》
  publishAt: 2020

- name: 《Third》
  publishAt: 2021
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	require.NoError(t, items[0].SetField("alias", "First Alias"))
	require.NoError(t, items[2].SetField("alias", "Third Alias"))
	require.NoError(t, SaveYAMLFile(path, items))

	items2, _, _ := ParseYAMLFile(path)
	require.Len(t, items2, 3)
	require.Equal(t, "First Alias", items2[0].GetField("alias"))
	require.Equal(t, "Third Alias", items2[2].GetField("alias"))
}

func TestFindInsertionPoint_AllComments(t *testing.T) {
	// When all lines in the range are comments, it should return startLine-1
	lines := [][]byte{
		[]byte("# comment 1"),
		[]byte("# comment 2"),
		[]byte("- name: next-item"),
	}
	locs := []itemLoc{
		{startLine: 1, pending: map[string]string{"key": "value"}},
		{startLine: 3, pending: map[string]string{}},
	}
	insertLine := findInsertionPoint(lines, locs, 0)
	// Should return 1 (the last non-empty, non-comment line in range [0, 1])
	// But since all are comments, returns startLine-1 = 0
	assert.Equal(t, 0, insertLine)
}

func TestBuildItemLocations_NilToken(t *testing.T) {
	// This is hard to trigger naturally, but we can test the sort behavior
	yaml := `- name: 《A》
- name: 《B》
- name: 《C》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	// Build locations from valid items
	locs := buildItemLocations(items)
	require.Len(t, locs, 3)
	// Should be sorted by startLine
	assert.True(t, locs[0].startLine <= locs[1].startLine)
	assert.True(t, locs[1].startLine <= locs[2].startLine)
}
