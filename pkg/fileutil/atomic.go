package fileutil

import (
	"fmt"
	"os"

	"github.com/google/renameio/v2"
)

// AtomicWriteFile writes data to path via a same-directory temporary file and
// atomically replaces the destination when the write succeeds.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err := EnsureFileDir(path); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if err := renameio.WriteFile(path, data, perm, renameio.WithStaticPermissions(perm)); err != nil {
		return fmt.Errorf("atomic write %s: %w", path, err)
	}

	return nil
}
