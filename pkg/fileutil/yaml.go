package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsYAMLFileName reports whether name is a visible .yml or .yaml file name.
func IsYAMLFileName(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") {
		return false
	}

	switch strings.ToLower(filepath.Ext(base)) {
	case ".yml", ".yaml":
		return true
	default:
		return false
	}
}

// ListYAMLFiles returns visible YAML files directly under dir.
func ListYAMLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !IsYAMLFileName(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	slices.Sort(files)

	return files, nil
}

// ListYAMLFilesRecursive returns visible YAML files under root, sorted by path.
func ListYAMLFilesRecursive(root string) ([]string, error) {
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
		if IsYAMLFileName(path) {
			files = append(files, path)
		}
	}
	slices.Sort(files)

	return files, nil
}
