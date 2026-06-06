package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileCreatesParentAndWritesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")

	if err := AtomicWriteFile(path, []byte(`{"ok":true}`), FilePermPrivate); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data = %q", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != FilePermPrivate {
		t.Fatalf("perm = %v, want %v", got, FilePermPrivate)
	}
}
