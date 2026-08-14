package goods

import (
	"fmt"

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

// RunCheckWithOptions validates goods YAML against the embedded goods JSON
// Schema.
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	sch, err := schemacheck.CompileBytes(schema.Goods)
	if err != nil {
		return nil, fmt.Errorf("compile goods schema: %w", err)
	}

	files, err := fileutil.ListYAMLFilesRecursive(path, fileutil.ListOptions{IncludeHidden: opts.IncludeHidden})
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
