package dotfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WalkNixFiles walks scope dirs under root, parses each .nix file,
// and calls fn(category, pkg) for every package reference found.
// Returns on first fn error or walk/read error.
func WalkNixFiles(root string, scope []string, fn func(category, pkg string) error) error {
	for _, scopeRel := range scope {
		scopeAbs := filepath.Join(root, scopeRel)
		if _, err := os.Stat(scopeAbs); os.IsNotExist(err) {
			continue
		}
		if err := walkScopeDir(root, scopeAbs, fn); err != nil {
			return fmt.Errorf("walk %s: %w", scopeAbs, err)
		}
	}
	return nil
}

// walkScopeDir walks a single scope directory and calls fn for each package reference.
func walkScopeDir(root, scopeAbs string, fn func(category, pkg string) error) error {
	return filepath.WalkDir(scopeAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".nix") {
			return nil
		}
		return processNixFile(root, path, fn)
	})
}

// processNixFile parses a single .nix file and calls fn for each package reference.
func processNixFile(root, path string, fn func(category, pkg string) error) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("rel %s: %w", path, err)
	}
	cat := categoryFromFile(rel)
	if cat == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	for _, pkg := range parseNixRefs(string(data)) {
		if err := fn(cat, pkg); err != nil {
			return fmt.Errorf("fn(%s, %s): %w", cat, pkg, err)
		}
	}
	return nil
}
