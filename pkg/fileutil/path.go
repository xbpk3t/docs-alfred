package fileutil

import (
	"path/filepath"
	"strings"
)

// PathWithin reports whether path is inside dir. dir must exist and be
// resolvable. Both sides are symlink-resolved before the comparison — a
// lexical check alone lets a symlink inside dir point at a file outside it.
func PathWithin(dir, path string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	resolvedDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	resolvedPath, ok := resolvePathTail(absPath)
	if !ok {
		return false
	}
	rel, err := filepath.Rel(resolvedDir, resolvedPath)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolvePathTail symlink-resolves the longest existing prefix of path and
// rejoins the missing tail lexically. EvalSymlinks alone rejects a path whose
// final component does not exist yet, and a plain lexical fallback would hide
// a symlink escape among the unresolved components.
func resolvePathTail(path string) (string, bool) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, true
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", false // reached the filesystem root without resolving
	}
	resolvedParent, ok := resolvePathTail(parent)
	if !ok {
		return "", false
	}

	return filepath.Join(resolvedParent, filepath.Base(path)), true
}
