package dotfiles

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// NixDiff holds the structured nix diff result.
type NixDiff struct {
	GhOnly        map[string][]string `json:"ghOnly"`
	DfOnly        map[string][]string `json:"dfOnly"`
	CrossCategory map[string]CrossPkg `json:"crossCategory"`
	Issues        []checkutil.Issue   `json:"issues"`
	Shared        int                 `json:"shared"`
}

type CrossPkg struct {
	GHCats []string `json:"ghCats"`
	DFCats []string `json:"dfCats"`
}

func (d *NixDiff) Summary() map[string]any {
	return map[string]any{
		"ghOnly":        len(d.GhOnly),
		"dfOnly":        len(d.DfOnly),
		"crossCategory": len(d.CrossCategory),
		"shared":        d.Shared,
	}
}

// CatDiff holds the category-level diff result.
type CatDiff struct {
	Shared []string
	DfOnly []string
	GhOnly []string
	Issues []checkutil.Issue
}

// DiffResult holds the combined diff result.
type DiffResult struct {
	Summary  map[string]any
	Category CatDiff
	Issues   []checkutil.Issue
	Nix      NixDiff
}

// DiffCategories compares two category lists and returns the diff.
func DiffCategories(ghCats, dfCats []string) CatDiff {
	ghSet := make(map[string]bool, len(ghCats))
	for _, c := range ghCats {
		ghSet[c] = true
	}
	dfSet := make(map[string]bool, len(dfCats))
	for _, c := range dfCats {
		dfSet[c] = true
	}

	var diff CatDiff
	allCats := make(map[string]bool)
	for c := range ghSet {
		allCats[c] = true
	}
	for c := range dfSet {
		allCats[c] = true
	}

	for cat := range allCats {
		inGh := ghSet[cat]
		inDf := dfSet[cat]

		switch {
		case inGh && inDf:
			diff.Shared = append(diff.Shared, cat)
		case inDf && !inGh:
			diff.DfOnly = append(diff.DfOnly, cat)
			diff.Issues = append(diff.Issues, checkutil.Issue{
				File:     "category:" + cat,
				Severity: checkutil.SeverityError,
				Message:  "exists in dotfiles but not in data/gh/ — add a gh entry or remove the directory",
			})
		case inGh && !inDf:
			diff.GhOnly = append(diff.GhOnly, cat)
			diff.Issues = append(diff.Issues, checkutil.Issue{
				File:     "category:" + cat,
				Severity: checkutil.SeverityError,
				Message:  "exists in data/gh/ but not in dotfiles — add isDotfiles: false to its types if intentional",
			})
		}
	}

	sort.Strings(diff.Shared)
	sort.Strings(diff.DfOnly)
	sort.Strings(diff.GhOnly)
	sort.Slice(diff.Issues, func(i, j int) bool {
		return diff.Issues[i].Message < diff.Issues[j].Message
	})

	return diff
}

// DiffNix compares GH and dotfiles nix package data.
// ghMap and dfMap are pkg → categories maps.
// falsePkgs is the set of packages marked isDotfiles: false.
// selfBuilt is the set of self-built packages to exclude.
func DiffNix(ghMap, dfMap map[string][]string, falsePkgs, selfBuilt map[string]bool) NixDiff {
	pkgState := make(map[string]*pkgPresence)
	for pkg, cats := range ghMap {
		if selfBuilt[pkg] {
			continue
		}
		if pkgState[pkg] == nil {
			pkgState[pkg] = &pkgPresence{}
		}
		pkgState[pkg].GHCats = cats
	}
	for pkg, cats := range dfMap {
		if selfBuilt[pkg] {
			continue
		}
		if pkgState[pkg] == nil {
			pkgState[pkg] = &pkgPresence{}
		}
		pkgState[pkg].DFCats = cats
	}

	// Apply prefix filter for package set namespaces
	prefixes := []string{
		"typstPackages.", "gnomeExtensions.", "vscode-extensions.",
		"yaziPlugins.",
	}
	for pkg := range pkgState {
		for _, prefix := range prefixes {
			if strings.HasPrefix(pkg, prefix) {
				delete(pkgState, pkg)
				break
			}
		}
	}

	return *classify(pkgState, falsePkgs)
}

// MergeResult combines category and nix diffs into a single result.
func MergeResult(cat *CatDiff, nix NixDiff) *DiffResult {
	issues := make([]checkutil.Issue, 0, len(cat.Issues)+len(nix.Issues))
	issues = append(issues, cat.Issues...)
	issues = append(issues, nix.Issues...)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Message < issues[j].Message
	})

	summary := map[string]any{
		"shared":      len(cat.Shared),
		"dfOnly":      len(cat.DfOnly),
		"ghOnly":      len(cat.GhOnly),
		"nixGhOnly":   len(nix.GhOnly),
		"nixDfOnly":   len(nix.DfOnly),
		"nixCrossCat": len(nix.CrossCategory),
		"nixShared":   nix.Shared,
	}

	return &DiffResult{
		Issues:   issues,
		Category: *cat,
		Nix:      nix,
		Summary:  summary,
	}
}

// FormatCompact returns a one-line summary for text output.
func FormatCompact(r *DiffResult) string {
	return fmt.Sprintf(
		"categories shared=%d df-only=%d gh-only=%d | nix gh-only=%d df-only=%d\n",
		len(r.Category.Shared), len(r.Category.DfOnly), len(r.Category.GhOnly),
		len(r.Nix.GhOnly), len(r.Nix.DfOnly),
	)
}

// pkgPresence tracks which categories a package appears in on each side.
type pkgPresence struct {
	GHCats []string
	DFCats []string
}

// classify partitions pkgState into gh-only, df-only, shared, and cross-category.
func classify(pkgState map[string]*pkgPresence, dotfilesFalsePkgs map[string]bool) *NixDiff {
	diff := &NixDiff{
		GhOnly:        make(map[string][]string),
		DfOnly:        make(map[string][]string),
		CrossCategory: make(map[string]CrossPkg),
	}

	for pkg, p := range pkgState {
		sort.Strings(p.GHCats)
		sort.Strings(p.DFCats)

		switch {
		case len(p.GHCats) > 0 && len(p.DFCats) == 0:
			if dotfilesFalsePkgs[pkg] {
				diff.Shared++
			} else {
				diff.GhOnly[pkg] = p.GHCats
				diff.Issues = append(diff.Issues, checkutil.Issue{
					File:     "data/gh",
					Severity: checkutil.SeverityError,
					Message:  fmt.Sprintf("gh-only: %s (data/gh: %s)", pkg, strings.Join(p.GHCats, ", ")),
				})
			}

		case len(p.DFCats) > 0 && len(p.GHCats) == 0:
			diff.DfOnly[pkg] = p.DFCats
			diff.Issues = append(diff.Issues, checkutil.Issue{
				File:     "dotfiles",
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("df-only: %s (dotfiles: %s)", pkg, strings.Join(p.DFCats, ", ")),
			})

		case len(p.GHCats) > 0 && len(p.DFCats) > 0:
			if hasOverlap(p.GHCats, p.DFCats) {
				diff.Shared++
			} else {
				diff.CrossCategory[pkg] = CrossPkg{GHCats: p.GHCats, DFCats: p.DFCats}
				diff.Issues = append(diff.Issues, checkutil.Issue{
					File:     "cross-category",
					Severity: checkutil.SeverityError,
					Message:  fmt.Sprintf("cross-category: %s (data/gh: %s, dotfiles: %s)", pkg, strings.Join(p.GHCats, ", "), strings.Join(p.DFCats, ", ")),
				})
			}
		}
	}

	sort.Slice(diff.Issues, func(i, j int) bool {
		return diff.Issues[i].Message < diff.Issues[j].Message
	})

	return diff
}

func hasOverlap(a, b []string) bool {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if seen[s] {
			return true
		}
	}
	return false
}

// FilterGhOnlyCategories removes GH-only categories where all YAML types
// have isDotfiles: false (intentional exclusion).
// Returns the filtered CatDiff and any error from YAML parsing.
func FilterGhOnlyCategories(diff *CatDiff, ghDir string) (*CatDiff, error) {
	filtered := &CatDiff{
		Shared: diff.Shared,
		DfOnly: diff.DfOnly,
	}

	// Check each gh-only category once
	excluded := make(map[string]bool)
	for _, cat := range diff.GhOnly {
		allNoDf, err := allYAMLNoDotfiles(filepath.Join(ghDir, cat))
		if err != nil {
			return filtered, fmt.Errorf("check gh category %s: %w", cat, err)
		}
		if allNoDf {
			excluded[cat] = true
		} else {
			filtered.GhOnly = append(filtered.GhOnly, cat)
		}
	}

	// Rebuild issues: keep non-excluded gh-only issues
	for _, iss := range diff.Issues {
		cat := strings.TrimPrefix(iss.File, "category:")
		if iss.File == "category:"+cat && excluded[cat] {
			continue
		}
		filtered.Issues = append(filtered.Issues, iss)
	}

	sort.Strings(filtered.GhOnly)
	sort.Slice(filtered.Issues, func(i, j int) bool {
		return filtered.Issues[i].Message < filtered.Issues[j].Message
	})

	return filtered, nil
}

// allYAMLNoDotfiles checks if all YAML files in a directory have isDotfiles: false.
func allYAMLNoDotfiles(dir string) (bool, error) {
	files, err := fileutil.ListYAMLFiles(dir)
	if err != nil {
		return false, fmt.Errorf("list yaml files in %s: %w", dir, err)
	}
	for _, file := range files {
		ok, err := yamlFileNoDotfiles(file)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// yamlFileNoDotfiles checks if all YAML documents in a file have isDotfiles: false.
func yamlFileNoDotfiles(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc []map[string]any
		err := decoder.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return false, fmt.Errorf("decode %s: %w", path, err)
		}
		for _, section := range doc {
			isDotfiles, ok := section["isDotfiles"].(bool)
			if !ok || isDotfiles {
				return false, nil
			}
		}
	}
	return true, nil
}
