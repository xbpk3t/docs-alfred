package dotfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotfilesNixData returns pkg → categories map from dotfiles.
func LoadDotfilesNixData(root string, scope []string) (map[string][]string, error) {
	result := make(map[string][]string)
	err := WalkNixFiles(root, scope, func(cat, pkg string) error {
		result[pkg] = append(result[pkg], cat)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// LoadDotfilesCategories returns category list from dotfiles home/base and home/core dirs.
func LoadDotfilesCategories(root string) ([]string, error) {
	cats := make(map[string]bool)
	for _, sub := range []string{"home/base", "home/core"} {
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				cats[e.Name()] = true
			}
		}
	}
	result := make([]string, 0, len(cats))
	for c := range cats {
		result = append(result, c)
	}
	return result, nil
}

// LoadSelfBuiltPkgs reads generated.json and returns a set of self-built package names.
// Returns error if the file exists but cannot be parsed.
func LoadSelfBuiltPkgs(path string) (map[string]bool, error) {
	result := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pkgs map[string]any
	if err := json.Unmarshal(data, &pkgs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for name := range pkgs {
		result[name] = true
	}
	return result, nil
}
