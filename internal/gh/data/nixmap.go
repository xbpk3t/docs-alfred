package ghdata

import (
	"strings"

	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
)

// loadRepos loads and parses all GH config repos from ghRoot.
func loadRepos(ghRoot string) (ghindex.Repos, error) {
	configs, err := ghindex.LoadConfigReposFromDir(ghRoot)
	if err != nil {
		return nil, err
	}
	return configs.ToRepos(), nil
}

// LoadNixData loads GH data in one pass and returns the nix map plus the
// isDotfiles:false escape-hatch set. Prefer this over calling GHNixMap and
// IsDotfilesFalsePkgs separately when you need both.
func LoadNixData(ghRoot string) (ghMap map[string]map[string]bool, falsePkgs map[string]bool, err error) {
	repos, err := loadRepos(ghRoot)
	if err != nil {
		return nil, nil, err
	}

	ghMap = make(map[string]map[string]bool)
	falsePkgs = make(map[string]bool)

	for _, r := range repos {
		if !ghindex.HasNix(r) {
			continue
		}
		short := extractNixShort(r.NixURL)
		if short == "" {
			continue
		}
		cat := r.Tag
		if cat == "" {
			cat = r.Type
		}
		if ghMap[cat] == nil {
			ghMap[cat] = make(map[string]bool)
		}
		ghMap[cat][short] = true

		if r.IsDotfiles != nil && !*r.IsDotfiles {
			falsePkgs[short] = true
		}
	}
	return ghMap, falsePkgs, nil
}

func extractNixShort(nixURL string) string {
	// Take the last path segment after the final /
	if idx := strings.LastIndex(nixURL, "/"); idx >= 0 && idx < len(nixURL)-1 {
		seg := nixURL[idx+1:]
		// For dotted identifiers (e.g., "networking.nftables", "programs.ssh"),
		// take only the last component as the package short name
		if dot := strings.LastIndex(seg, "."); dot >= 0 {
			return strings.ToLower(seg[dot+1:])
		}
		return strings.ToLower(seg)
	}
	return ""
}
