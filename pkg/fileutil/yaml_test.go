package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yml"))
	writeTestFile(t, filepath.Join(dir, "b.yaml"))
	writeTestFile(t, filepath.Join(dir, "c.txt"))
	writeTestFile(t, filepath.Join(dir, ".hidden.yml"))
	if err := os.Mkdir(filepath.Join(dir, "subdir"), DirPerm); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "subdir", "nested.yml"))

	files, err := ListYAMLFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(dir, "a.yml"), filepath.Join(dir, "b.yaml")}
	assertStringSlicesEqual(t, files, want)
}

func TestListYAMLFilesRecursive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yml"))
	writeTestFile(t, filepath.Join(dir, ".hidden.yml"))
	if err := os.Mkdir(filepath.Join(dir, "subdir"), DirPerm); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "subdir", "nested.yaml"))

	files, err := ListYAMLFilesRecursive(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(dir, "a.yml"), filepath.Join(dir, "subdir", "nested.yaml")}
	assertStringSlicesEqual(t, files, want)
}

func TestReadAndMergeYAMLFilesRecursiveUsesVisibleSortedYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, filepath.Join(dir, "b.yaml"), "b: true\n")
	writeContent(t, filepath.Join(dir, ".hidden.yml"), "hidden: true\n")
	writeContent(t, filepath.Join(dir, "note.txt"), "note: true\n")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), DirPerm); err != nil {
		t.Fatal(err)
	}
	writeContent(t, filepath.Join(dir, "subdir", "a.yml"), "a: true\n")

	var seen []string
	data, err := ReadAndMergeYAMLFilesRecursive(dir, func(filename string) {
		seen = append(seen, filename)
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := string(data); got != "b: true\na: true\n" {
		t.Fatalf("merged data = %q", got)
	}
	assertStringSlicesEqual(t, seen, []string{"b.yaml", "a.yml"})
	if strings.Contains(string(data), "hidden") || strings.Contains(string(data), "note") {
		t.Fatalf("merged unexpected file content: %q", string(data))
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	writeContent(t, path, "key: value\n")
}

func writeContent(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), FilePermPrivate); err != nil {
		t.Fatal(err)
	}
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d files %v, want %d files %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q; all got %v", i, got[i], want[i], got)
		}
	}
}
