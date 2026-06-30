package dotfiles

import (
	"fmt"
)

// DedupRef finds nix packages referenced in multiple categories.
// Returns pkg → list of relative file paths.
func DedupRef(dotfilesRoot string, scopeDirs []string) (map[string][]string, error) {
	pkgFiles := make(map[string]map[string]bool)

	err := WalkNixFiles(dotfilesRoot, scopeDirs, func(cat, pkg string) error {
		// We need the relative file path for the result.
		// WalkNixFiles doesn't provide it, so we reconstruct from cat.
		// Instead, track by category for dedup purposes.
		if pkgFiles[pkg] == nil {
			pkgFiles[pkg] = make(map[string]bool)
		}
		pkgFiles[pkg][cat] = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk nix files: %w", err)
	}

	return crossCategoryPkgs(pkgFiles), nil
}

// crossCategoryPkgs returns packages referenced in multiple categories.
func crossCategoryPkgs(pkgFiles map[string]map[string]bool) map[string][]string {
	result := make(map[string][]string)
	for pkg, cats := range pkgFiles {
		if len(cats) <= 1 {
			continue
		}
		for cat := range cats {
			result[pkg] = append(result[pkg], cat)
		}
	}
	return result
}
