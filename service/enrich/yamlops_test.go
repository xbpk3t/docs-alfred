package enrich

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test yaml: %v", err)
	}
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
	if err != nil {
		t.Fatalf("ParseYAMLFile: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].GetName() != "《Item One》" {
		t.Errorf("item 0 name = %q, want '《Item One》'", items[0].GetName())
	}
	if items[1].GetName() != "《Item Two》" {
		t.Errorf("item 1 name = %q, want '《Item Two》'", items[1].GetName())
	}
}

func TestGetField(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
  author: Some Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	if items[0].GetField("name") != "《Test》" {
		t.Errorf("name field: got %q", items[0].GetField("name"))
	}
	if items[0].GetField("publishAt") != "2020" {
		t.Errorf("publishAt field: got %q", items[0].GetField("publishAt"))
	}
	if items[0].GetField("nonexistent") != "" {
		t.Errorf("nonexistent field: got %q", items[0].GetField("nonexistent"))
	}
}

func TestFieldExists(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
  emptyField: ""
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	if !items[0].FieldExists("name") {
		t.Error("name should exist")
	}
	if !items[0].FieldExists("publishAt") {
		t.Error("publishAt should exist")
	}
	if items[0].FieldExists("nonexistent") {
		t.Error("nonexistent should not exist")
	}
	if items[0].FieldExists("emptyField") {
		t.Error("emptyField should not exist")
	}
}

func TestSetField(t *testing.T) {
	yaml := `- name: 《Test》
  author: Old Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	// Set new field
	if err := items[0].SetField("publishAt", "2023"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	// Save and verify by re-parsing
	if err := SaveYAMLFile(path, items); err != nil {
		t.Fatalf("SaveYAMLFile: %v", err)
	}

	items2, _, _ := ParseYAMLFile(path)
	if items2[0].GetField("publishAt") != "2023" {
		t.Errorf("after set publishAt: got %q", items2[0].GetField("publishAt"))
	}
}

func TestSetFieldPreservesComments(t *testing.T) {
	yaml := `# Top-level comment
- name: 《Test》
  author: Author
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	if err := items[0].SetField("publishAt", "2023"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	if err := SaveYAMLFile(path, items); err != nil {
		t.Fatalf("SaveYAMLFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if len(content) == 0 {
		t.Fatal("file is empty after write")
	}
}

func TestSetFieldEmptyValue(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	// Empty value should be rejected
	if err := items[0].SetField("publishAt", ""); err == nil {
		t.Error("expected error for empty value")
	}
}

func TestMultiItemFieldOperations(t *testing.T) {
	yaml := `- name: 《First》
  publishAt: 2019

- name: 《Second》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}

	if !items[0].FieldExists("publishAt") {
		t.Error("item 0 should have publishAt")
	}
	if items[1].FieldExists("publishAt") {
		t.Error("item 1 should not have publishAt")
	}

	if err := items[1].SetField("publishAt", "2020"); err != nil {
		t.Fatalf("SetField: %v", err)
	}
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
	if err != nil {
		t.Fatalf("ParseYAMLFile: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
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
	if err != nil {
		t.Fatalf("ParseYAMLFile: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].GetName() != "《Item One》" {
		t.Errorf("name = %q", items[0].GetName())
	}
}

func TestParseYAMLFileEmpty(t *testing.T) {
	path := writeTestYAML(t, "")
	items, _, err := ParseYAMLFile(path)
	if err != nil {
		t.Fatalf("ParseYAMLFile: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty file, got %d", len(items))
	}
}

func TestGetPublishAtInteger(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2020
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	if items[0].GetPublishAt() != "2020" {
		t.Errorf("GetPublishAt = %q, want 2020", items[0].GetPublishAt())
	}
}

func TestGetPublishAtString(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: "2020"
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	if items[0].GetPublishAt() != "2020" {
		t.Errorf("GetPublishAt = %q, want 2020", items[0].GetPublishAt())
	}
}

func TestGetPublishAtMissing(t *testing.T) {
	yaml := `- name: 《Test》
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)
	if items[0].GetPublishAt() != "" {
		t.Errorf("GetPublishAt = %q, want empty", items[0].GetPublishAt())
	}
}

func TestSaveSetFieldOnMultiItemWithChinese(t *testing.T) {
	yaml := `- name: 《影响力》
  author: 西奥迪尼

- name: 《毛选》
  author: 毛泽东
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	if err := items[0].SetField("publishAt", "1984"); err != nil {
		t.Fatalf("SetField item 0: %v", err)
	}
	if err := items[1].SetField("publishAt", "1960"); err != nil {
		t.Fatalf("SetField item 1: %v", err)
	}

	if err := SaveYAMLFile(path, items); err != nil {
		t.Fatalf("SaveYAMLFile: %v", err)
	}

	items2, _, _ := ParseYAMLFile(path)
	if items2[0].GetField("publishAt") != "1984" {
		t.Errorf("item 0 publishAt: got %q, want 1984", items2[0].GetField("publishAt"))
	}
	if items2[1].GetField("publishAt") != "1960" {
		t.Errorf("item 1 publishAt: got %q, want 1960", items2[1].GetField("publishAt"))
	}
}

func TestSaveSetFieldOnItemWithExistingFields(t *testing.T) {
	yaml := `- name: 《Test》
  publishAt: 2019
  score: 4
`
	path := writeTestYAML(t, yaml)
	items, _, _ := ParseYAMLFile(path)

	// Set alias (new field)
	if err := items[0].SetField("alias", "English Title"); err != nil {
		t.Fatalf("SetField alias: %v", err)
	}

	if err := SaveYAMLFile(path, items); err != nil {
		t.Fatalf("SaveYAMLFile: %v", err)
	}

	items2, _, _ := ParseYAMLFile(path)
	if items2[0].GetField("alias") != "English Title" {
		t.Errorf("alias: got %q, want 'English Title'", items2[0].GetField("alias"))
	}
	// Original fields should be preserved
	if items2[0].GetField("publishAt") != "2019" {
		t.Errorf("publishAt changed: got %q", items2[0].GetField("publishAt"))
	}
}
