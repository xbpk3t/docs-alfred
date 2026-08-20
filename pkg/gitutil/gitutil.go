// Package gitutil provides go-git based utilities for common git operations,
// replacing shell-out calls to the git CLI.
//
// Supported operations:
//   - Repo root detection (git rev-parse --show-toplevel)
package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindRepoRoot returns the root directory of the git repository containing
// the given path. Equivalent to `git rev-parse --show-toplevel`.
func FindRepoRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Walk up from absPath to find a .git directory
	dir := absPath
	for {
		gitDir := filepath.Join(dir, ".git")
		info, statErr := os.Stat(gitDir)
		if statErr == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no git repository found from %s", path)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}

	return len(splitLines(s))
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}

	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
