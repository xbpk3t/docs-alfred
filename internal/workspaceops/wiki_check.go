package workspaceops

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// WikiCheckInput holds input for wiki check.
type WikiCheckInput struct {
	GhRoot   string
	WikiRoot string
}

// WikiCheckResult holds wiki check results.
type WikiCheckResult struct {
	Issues           []checkutil.Issue
	ExpectedWikiDirs []string
	ActualWikiDirs   []string
	MissingWikiDirs  []string
	ExtraWikiDirs    []string
}

// Summary returns count-oriented wiki check details for structured output.
func (r *WikiCheckResult) Summary() map[string]any {
	return map[string]any{
		"expectedWikiDirs": len(r.ExpectedWikiDirs),
		"actualWikiDirs":   len(r.ActualWikiDirs),
		"missingWikiDirs":  len(r.MissingWikiDirs),
		"extraWikiDirs":    len(r.ExtraWikiDirs),
	}
}

// RunWikiCheck checks that wiki/ and data/gh/ have matching folder structures.
func RunWikiCheck(input WikiCheckInput) (*WikiCheckResult, error) {
	slog.Info("Running wiki check", "gh-root", input.GhRoot, "wiki-root", input.WikiRoot)

	expectedDirs, err := collectExpectedDirs(input.GhRoot)
	if err != nil {
		return nil, err
	}

	actualDirs, err := collectActualWikiDirs(input.WikiRoot)
	if err != nil {
		return nil, err
	}

	expectedSet := toSet(expectedDirs)
	actualSet := toSet(actualDirs)

	missing, extra := computeDirDiff(expectedDirs, actualDirs, expectedSet, actualSet)
	extra = filterContainerDirs(extra, expectedDirs)
	issues := buildWikiIssues(missing, extra)

	// Append OKF v0.1 frontmatter check.
	okfIssues, err := RunWikiCheckOKF(input.WikiRoot)
	if err != nil {
		return nil, err
	}
	issues = append(issues, okfIssues...)

	return &WikiCheckResult{
		Issues:           issues,
		ExpectedWikiDirs: expectedDirs,
		ActualWikiDirs:   actualDirs,
		MissingWikiDirs:  missing,
		ExtraWikiDirs:    extra,
	}, nil
}

// toSet converts a string slice to a lookup set.
func toSet(dirs []string) map[string]bool {
	set := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		set[d] = true
	}

	return set
}

// collectExpectedDirs derives expected wiki dir paths from data/gh YAML file stems.
func collectExpectedDirs(ghRoot string) ([]string, error) {
	yamlFiles, err := fileutil.ListYAMLFilesRecursive(ghRoot)
	if err != nil {
		return nil, fmt.Errorf("list YAML files: %w", err)
	}

	set := make(map[string]bool)
	var dirs []string
	for _, f := range yamlFiles {
		rel, errRel := filepath.Rel(ghRoot, f)
		if errRel != nil {
			continue
		}
		stem := strings.TrimSuffix(rel, filepath.Ext(rel))
		if strings.HasPrefix(stem, ".") {
			continue
		}
		if !set[stem] {
			set[stem] = true
			dirs = append(dirs, stem)
		}
	}
	slices.Sort(dirs)

	return dirs, nil
}

// collectActualWikiDirs collects depth-1 and depth-2 wiki dirs, skipping hidden entries.
func collectActualWikiDirs(wikiRoot string) ([]string, error) {
	set := make(map[string]bool)
	var dirs []string

	entries, err := os.ReadDir(wikiRoot)
	if err != nil {
		return nil, fmt.Errorf("read wiki root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		d1 := entry.Name()
		addDir(d1, set, &dirs)
		collectWikiDepth2Dirs(wikiRoot, d1, set, &dirs)
	}
	slices.Sort(dirs)

	return dirs, nil
}

// collectWikiDepth2Dirs appends depth-2 wiki directories under parent to dirs.
func collectWikiDepth2Dirs(wikiRoot, d1 string, set map[string]bool, dirs *[]string) {
	subEntries, err := os.ReadDir(filepath.Join(wikiRoot, d1))
	if err != nil {
		return
	}
	for _, subEntry := range subEntries {
		if !subEntry.IsDir() {
			continue
		}
		if strings.HasPrefix(subEntry.Name(), ".") {
			continue
		}
		d2 := d1 + "/" + subEntry.Name()
		addDir(d2, set, dirs)
	}
}

// addDir adds dir to set and appends it to dirs if not already present.
func addDir(dir string, set map[string]bool, dirs *[]string) {
	if set[dir] {
		return
	}
	set[dir] = true
	*dirs = append(*dirs, dir)
}

// computeDirDiff returns missing and extra dirs between expected and actual sets.
//
//nolint:nonamedreturns // named returns clarify which slice is missing vs extra
func computeDirDiff(expectedDirs, actualDirs []string, expectedSet, actualSet map[string]bool) (missing, extra []string) {
	for _, d := range expectedDirs {
		if !actualSet[d] {
			missing = append(missing, d)
		}
	}
	for _, d := range actualDirs {
		if !expectedSet[d] {
			extra = append(extra, d)
		}
	}

	return
}

// filterContainerDirs removes depth-1 dirs that are parent containers of expected dirs.
func filterContainerDirs(extra, expectedDirs []string) []string {
	var filtered []string
	for _, d := range extra {
		if !strings.Contains(d, "/") {
			isContainer := false
			for _, ed := range expectedDirs {
				if strings.HasPrefix(ed, d+"/") {
					isContainer = true

					break
				}
			}
			if isContainer {
				continue
			}
		}
		filtered = append(filtered, d)
	}

	return filtered
}

// buildWikiIssues creates error-severity issues for missing and extra wiki dirs.
func buildWikiIssues(missing, extra []string) []checkutil.Issue {
	var issues []checkutil.Issue
	for _, d := range missing {
		issues = append(issues, checkutil.Issue{
			File:     d,
			Severity: checkutil.SeverityError,
			Message:  "missing wiki dir: " + d,
		})
	}
	for _, d := range extra {
		issues = append(issues, checkutil.Issue{
			File:     d,
			Severity: checkutil.SeverityError,
			Message:  "extra wiki dir without data/gh YAML: " + d,
		})
	}

	return issues
}
