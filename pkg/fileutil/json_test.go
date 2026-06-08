package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteJSONFileAndReadJSONFile(t *testing.T) {
	type state struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := state{Name: "demo", Count: 2}

	if err := AtomicWriteJSONFile(path, want, FilePermPrivate); err != nil {
		t.Fatalf("AtomicWriteJSONFile() error = %v", err)
	}

	got, err := ReadJSONFile[state](path)
	if err != nil {
		t.Fatalf("ReadJSONFile() error = %v", err)
	}
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "{\n  \"name\": \"demo\",\n  \"count\": 2\n}" {
		t.Fatalf("json = %q", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if gotPerm := info.Mode().Perm(); gotPerm != FilePermPrivate {
		t.Fatalf("perm = %v, want %v", gotPerm, FilePermPrivate)
	}
}

func TestReadJSONFileMissingFile(t *testing.T) {
	_, err := ReadJSONFile[map[string]string](filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
}

func TestReadJSONFileInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"bad"`), FilePermPrivate); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := ReadJSONFile[map[string]string](path)
	if err == nil {
		t.Fatal("ReadJSONFile() error = nil, want parse error")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	got, err := UnmarshalJSON[map[string]int]([]byte(`{"count":2}`))
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if got["count"] != 2 {
		t.Fatalf("count = %d, want 2", got["count"])
	}
}

func TestUnmarshalJSONIntoPreservesExistingFields(t *testing.T) {
	type state struct {
		Version int               `json:"version"`
		Items   map[string]string `json:"items"`
	}

	got := state{Version: 1}
	if err := UnmarshalJSONInto([]byte(`{"items":{"a":"b"}}`), &got); err != nil {
		t.Fatalf("UnmarshalJSONInto() error = %v", err)
	}
	if got.Version != 1 {
		t.Fatalf("version = %d, want 1", got.Version)
	}
	if got.Items["a"] != "b" {
		t.Fatalf("items[a] = %q, want b", got.Items["a"])
	}
}
