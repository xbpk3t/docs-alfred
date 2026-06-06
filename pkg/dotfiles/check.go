package dotfiles

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// CheckResult holds the dotfiles check result.
type CheckResult struct {
	Issues      []checkutil.Issue
	SharedCount int
	DfOnlyCount int
	GhOnlyCount int
}

// RunCheck compares dotfiles categories with data/gh categories.
func RunCheck(dotfilesDir, ghDir string) (*CheckResult, error) {
	result := &CheckResult{}

	// Collect dotfiles categories from home/base and home/core
	basePath := filepath.Join(dotfilesDir, "home", "base")
	corePath := filepath.Join(dotfilesDir, "home", "core")

	var sources []struct {
		path   string
		prefix string
	}

	if info, err := os.Stat(basePath); err == nil && info.IsDir() {
		sources = append(sources, struct {
			path   string
			prefix string
		}{basePath, "home/base"})
	} else {
		result.Issues = append(result.Issues, checkutil.Issue{
			File: basePath, Severity: checkutil.SeverityError,
			Message: "home/base not found in dotfiles",
		})

		return result, nil
	}

	if info, err := os.Stat(corePath); err == nil && info.IsDir() {
		sources = append(sources, struct {
			path   string
			prefix string
		}{corePath, "home/core"})
	}

	// Collect categories
	dfCats := make(map[string]bool)
	for _, src := range sources {
		subdirs := listSubdirs(src.path)
		for _, d := range subdirs {
			dfCats[d] = true
		}
	}
	ghCats := listSubdirs(ghDir)

	compareCategories(result, dfCats, ghCats, sources, ghDir)

	return result, nil
}

func compareCategories(result *CheckResult, dfCats map[string]bool, ghCats []string, sources []struct {
	path   string
	prefix string
}, ghDir string) {
	// Compare
	allCats := make(map[string]bool)
	for c := range dfCats {
		allCats[c] = true
	}
	for _, c := range ghCats {
		allCats[c] = true
	}

	for cat := range allCats {
		hasDf := dfCats[cat]
		hasGh := contains(ghCats, cat)

		if hasDf && hasGh {
			result.SharedCount++
			checkDfGhOverlap(result, cat, sources, ghDir)
		} else if hasDf && !hasGh {
			result.DfOnlyCount++
			result.Issues = append(result.Issues, checkutil.Issue{
				File: "category:" + cat, Severity: checkutil.SeverityError,
				Message: "exists in dotfiles but not in data/gh/ — add a gh entry or remove the directory",
			})
		} else if !hasDf && hasGh {
			result.GhOnlyCount++
			checkGhOnlyCategory(result, cat, ghDir)
		}
	}
}

func checkGhOnlyCategory(result *CheckResult, cat, ghDir string) {
	ghCatPath := filepath.Join(ghDir, cat)
	if !hasAllTypesMarkedNoDotfiles(ghCatPath) {
		result.Issues = append(result.Issues, checkutil.Issue{
			File: "category:" + cat, Severity: checkutil.SeverityError,
			Message: "exists in data/gh/ but not in dotfiles — add isDotfiles: false to its types if intentional",
		})
	}
}

func checkDfGhOverlap(result *CheckResult, cat string, sources []struct {
	path   string
	prefix string
}, ghDir string) {
	ghCatPath := filepath.Join(ghDir, cat)
	dfHasContent := hasContentInSources(sources, cat)
	ghHasContent := hasYAMLFiles(ghCatPath)

	if !dfHasContent && !ghHasContent {
		result.Issues = append(result.Issues, checkutil.Issue{
			File: "category:" + cat, Severity: "warn",
			Message: "exists on both sides but has no real content",
		})
	}
}

func listSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}

	return dirs
}

func hasContentInSources(sources []struct {
	path   string
	prefix string
}, cat string,
) bool {
	for _, src := range sources {
		catPath := filepath.Join(src.path, cat)
		if hasContentNixFiles(catPath) {
			return true
		}
	}

	return false
}

func hasContentNixFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if hasContentNixFiles(filepath.Join(dir, e.Name())) {
				return true
			}
		} else if !e.IsDir() && strings.HasSuffix(e.Name(), ".nix") && e.Name() != "default.nix" {
			return true
		}
	}

	return false
}

func hasYAMLFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml")) {
			return true
		}
	}

	return false
}

func hasAllTypesMarkedNoDotfiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		if !isYAMLFileNoDotfiles(dir, e.Name()) {
			return false
		}
	}

	return true
}

func isYAMLFileNoDotfiles(dir, name string) bool {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc []map[string]any
		err := decoder.Decode(&doc)
		if err != nil {
			break
		}
		for _, section := range doc {
			isDotfiles, ok := section["isDotfiles"].(bool)
			if !ok || isDotfiles {
				return false
			}
		}
	}

	return true
}

func contains(slice []string, s string) bool {
	return slices.Contains(slice, s)
}
