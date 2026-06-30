package dotfiles

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	ghdata "github.com/xbpk3t/docs-alfred/internal/gh/data"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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

// pkgPresence tracks which categories a package appears in on each side.
type pkgPresence struct {
	GHCats []string
	DFCats []string
}

func RunNixDiff(ghRoot, dotfilesRoot string, scopeDirs []string) (*NixDiff, error) {
	rawGhMap, dotfilesFalsePkgs, err := ghdata.LoadNixData(ghRoot)
	if err != nil {
		return nil, err
	}

	dfNixMap, err := BuildNixMap(dotfilesRoot, scopeDirs)
	if err != nil {
		return nil, err
	}

	pkgState := buildPkgState(rawGhMap, dfNixMap)
	applyFilters(pkgState, dotfilesRoot)
	return classify(pkgState, dotfilesFalsePkgs), nil
}

// buildPkgState merges GH and dotfiles maps into a single pkg → presence map.
func buildPkgState(rawGhMap, dfNixMap map[string]map[string]bool) map[string]*pkgPresence {
	pkgState := make(map[string]*pkgPresence)

	for cat, pkgs := range rawGhMap {
		for pkg := range pkgs {
			if pkgState[pkg] == nil {
				pkgState[pkg] = &pkgPresence{}
			}
			pkgState[pkg].GHCats = append(pkgState[pkg].GHCats, cat)
		}
	}
	for cat, pkgs := range dfNixMap {
		for pkg := range pkgs {
			if pkgState[pkg] == nil {
				pkgState[pkg] = &pkgPresence{}
			}
			pkgState[pkg].DFCats = append(pkgState[pkg].DFCats, cat)
		}
	}

	return pkgState
}

// applyFilters removes self-built and namespaced packages from pkgState.
func applyFilters(pkgState map[string]*pkgPresence, dotfilesRoot string) {
	selfBuilt := LoadSelfBuiltPkgs(filepath.Join(dotfilesRoot, "pkgs", "_sources", "generated.json"))
	for pkg := range selfBuilt {
		delete(pkgState, pkg)
	}

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
