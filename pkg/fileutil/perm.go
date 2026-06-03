// Package fileutil provides file permission constants and utilities.
package fileutil

import "os"

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
