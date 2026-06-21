package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFileCreatesParentAndWritesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")

	require.NoError(t, AtomicWriteFile(path, []byte(`{"ok":true}`), FilePermPrivate))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, string(data))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, FilePermPrivate, info.Mode().Perm())
}
