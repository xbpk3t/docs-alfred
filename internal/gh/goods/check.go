package goods

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

// CheckResult holds goods validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// CheckOptions controls goods check behavior.
type CheckOptions struct {
	// IncludeHidden reports whether hidden (dot-prefixed) YAML files are checked.
	// By default hidden files are ignored.
	IncludeHidden bool
}

// RunCheck validates goods YAML syntax and structure.
func RunCheck(path string) (*CheckResult, error) {
	return RunCheckWithOptions(path, CheckOptions{})
}

// RunCheckWithOptions validates goods YAML against the shared gh JSON Schema
// (goods reuses the same section/topic/table/record structure as data/gh).
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	sch, err := schemacheck.CompileBytes(schema.Gh)
	if err != nil {
		return nil, fmt.Errorf("compile gh schema: %w", err)
	}

	files, err := collectYAMLFiles(path, opts.IncludeHidden)
	if err != nil {
		return nil, fmt.Errorf("list goods yaml under %s: %w", path, err)
	}

	var issues []checkutil.Issue
	for _, file := range files {
		// goods topics carry no kind; the gh-only kind-presence post-rule is
		// deliberately not wired here.
		issues = append(issues, schemacheck.CheckFile(file, sch, nil)...)
	}

	return &CheckResult{Issues: issues}, nil
}

// collectYAMLFiles lists goods YAML files under root. Hidden (dot-prefixed)
// files are included only when includeHidden is set.
func collectYAMLFiles(root string, includeHidden bool) ([]string, error) {
	if !includeHidden {
		return fileutil.ListYAMLFilesRecursive(root)
	}

	pattern := filepath.Join(root, "**", "*.{yml,yaml}")
	matches, err := doublestar.FilepathGlob(pattern, doublestar.WithFilesOnly())
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	return matches, nil
}
