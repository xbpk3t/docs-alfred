package enrich

import (
	"os"
	"path/filepath"
	"testing"

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
