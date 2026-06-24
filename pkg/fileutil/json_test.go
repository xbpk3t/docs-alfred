package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteJSONFileAndReadJSONFile(t *testing.T) {
	type state struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := state{Name: "demo", Count: 2}

	require.NoError(t, AtomicWriteJSONFile(path, want, FilePermPrivate))

	got, err := ReadJSONFile[state](path)
	require.NoError(t, err)
	require.Equal(t, want, got)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, "{\n  \"name\": \"demo\",\n  \"count\": 2\n}", string(data))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, FilePermPrivate, info.Mode().Perm())
}

func TestReadJSONFileMissingFile(t *testing.T) {
	_, err := ReadJSONFile[map[string]string](filepath.Join(t.TempDir(), "missing.json"))
	require.ErrorIs(t, err, os.ErrNotExist, "error should be os.ErrNotExist")
}

func TestReadJSONFileInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"bad"`), FilePermPrivate))

	_, err := ReadJSONFile[map[string]string](path)
	require.Error(t, err, "ReadJSONFile should return parse error")
}

func TestUnmarshalJSON(t *testing.T) {
	got, err := UnmarshalJSON[map[string]int]([]byte(`{"count":2}`))
	require.NoError(t, err)
	require.Equal(t, 2, got["count"])
}

func TestJSONUnmarshalPreservesExistingFields(t *testing.T) {
	type state struct {
		Items   map[string]string `json:"items"`
		Version int               `json:"version"`
	}

	got := state{Version: 1}
	require.NoError(t, json.Unmarshal([]byte(`{"items":{"a":"b"}}`), &got))
	require.Equal(t, 1, got.Version)
	require.Equal(t, "b", got.Items["a"])
}
