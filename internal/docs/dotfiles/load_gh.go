package dotfiles

import (
	"fmt"
	"os"
	"strings"

	ghdata "github.com/xbpk3t/docs-alfred/internal/gh/data"
)

// LoadGHNixData returns pkg → categories map from GH data.
func LoadGHNixData(ghRoot string) (map[string][]string, error) {
	rawMap, _, err := ghdata.LoadNixData(ghRoot)
	if err != nil {
		return nil, fmt.Errorf("load gh nix data: %w", err)
	}

	result := make(map[string][]string)
	for cat, pkgs := range rawMap {
		for pkg := range pkgs {
			result[pkg] = append(result[pkg], cat)
		}
	}
	return result, nil
}

// LoadGHCategories returns the list of category directories under ghDir.
func LoadGHCategories(ghDir string) ([]string, error) {
	entries, err := os.ReadDir(ghDir)
	if err != nil {
		return nil, fmt.Errorf("read gh dir %s: %w", ghDir, err)
	}
	var cats []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			cats = append(cats, e.Name())
		}
	}
	return cats, nil
}

// LoadGHFalsePkgs returns the set of packages marked isDotfiles: false in GH data.
func LoadGHFalsePkgs(ghRoot string) (map[string]bool, error) {
	_, falsePkgs, err := ghdata.LoadNixData(ghRoot)
	if err != nil {
		return nil, fmt.Errorf("load gh false pkgs: %w", err)
	}
	return falsePkgs, nil
}
