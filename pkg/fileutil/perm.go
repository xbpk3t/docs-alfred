// Package fileutil provides file and path utilities.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateOutputPath rejects paths that traverse above the working directory via "..".
func ValidateOutputPath(path string) error {
	if path == "" {
		return fmt.Errorf("output path must not be empty")
	}
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("output path %q escapes the working directory", path)
	}
	return nil
}

// Standard file permission constants.
const (
	// DirPerm is the default permission for directories (rwxr-x---).
	DirPerm os.FileMode = 0o750
	// FilePerm is the default permission for files (rw-r-----).
	FilePerm os.FileMode = 0o640
	// FilePermPrivate is the permission for private files (rw-------).
	FilePermPrivate os.FileMode = 0o600
	// FilePermShared is the permission for shared-writable files (rw-rw----).
	FilePermShared os.FileMode = 0o660
)

// EnsureDir creates a directory and all parent directories if they don't exist.
func EnsureDir(path string) error {
	if path == "" {
		return nil
	}

	return os.MkdirAll(path, DirPerm)
}

// EnsureFileDir creates the parent directory of a file path if it doesn't exist.
func EnsureFileDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "/" {
		return nil
	}

	return EnsureDir(dir)
}
