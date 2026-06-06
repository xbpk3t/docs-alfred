package fileutil

import (
	"os"
	"path/filepath"
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

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("key: value\n"), FilePermPrivate); err != nil {
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
