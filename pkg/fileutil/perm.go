// Package fileutil provides file and path utilities.
package fileutil

import (
	"os"
	"path/filepath"
	"strings"
)

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

// WriteFileSafe writes data to a file, creating parent directories as needed.
func WriteFileSafe(filePath string, data []byte, perm os.FileMode) error {
	if err := EnsureFileDir(filePath); err != nil {
		return err
	}

	return os.WriteFile(filePath, data, perm)
}

// IsSubPath checks that child is a sub-path of parent (path traversal protection).
// Returns true if child is within parent.
func IsSubPath(parent, child string) bool {
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	absChild, err := filepath.Abs(child)
	if err != nil {
		return false
	}

	absParent = filepath.Clean(absParent)
	absChild = filepath.Clean(absChild)

	if absParent == absChild {
		return true
	}

	return strings.HasPrefix(absChild, absParent+string(filepath.Separator))
}

// RelPathSafe resolves a relative path from base, ensuring no path traversal escape.
// Returns the resolved path or an error if the path attempts to escape base.
func RelPathSafe(base, target string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}

	var resolved string
	if filepath.IsAbs(target) {
		resolved = filepath.Clean(target)
	} else {
		resolved = filepath.Clean(filepath.Join(absBase, target))
	}

	if !IsSubPath(absBase, resolved) {
		return "", ErrPathTraversal
	}

	return resolved, nil
}

// ErrPathTraversal is returned when a path attempts to escape the base directory.
var ErrPathTraversal = &pathTraversalError{}

type pathTraversalError struct{}

func (e *pathTraversalError) Error() string {
	return "path traversal detected: target escapes base directory"
}
