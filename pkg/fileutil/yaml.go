package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// ListOptions controls file listing behavior.
type ListOptions struct {
	// IncludeHidden reports whether dot-prefixed (hidden) files are matched.
	// By default hidden files are excluded.
	IncludeHidden bool
}

// isYAMLFileName reports whether name is a .yml or .yaml file name.
// When includeHidden is false, dot-prefixed files are excluded.
func isYAMLFileName(name string, includeHidden bool) bool {
	base := filepath.Base(name)
	if !includeHidden && strings.HasPrefix(base, ".") {
		return false
	}

	switch strings.ToLower(filepath.Ext(base)) {
	case ".yml", ".yaml":
		return true
	default:
		return false
	}
}

// ListYAMLFiles returns YAML files directly under dir, sorted by name.
// Hidden (dot-prefixed) files are excluded by default; pass ListOptions{IncludeHidden: true} to include them.
func ListYAMLFiles(dir string, opts ...ListOptions) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFileName(entry.Name(), includeHidden(opts)) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(files)

	return files, nil
}

// ListYAMLFilesRecursive returns YAML files under root, sorted by path.
// Hidden (dot-prefixed) files are excluded by default; pass ListOptions{IncludeHidden: true} to include them.
func ListYAMLFilesRecursive(root string, opts ...ListOptions) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("root dir %s: %w", root, err)
	}

	pattern := filepath.Join(root, "**", "*.{yml,yaml}")
	matches, err := doublestar.FilepathGlob(pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, err
	}

	files := matches[:0]
	for _, path := range matches {
		if isYAMLFileName(path, includeHidden(opts)) {
			files = append(files, path)
		}
	}
	slices.Sort(files)

	return files, nil
}

// includeHidden reports whether any given option enables hidden-file matching.
func includeHidden(opts []ListOptions) bool {
	for _, o := range opts {
		if o.IncludeHidden {
			return true
		}
	}

	return false
}
