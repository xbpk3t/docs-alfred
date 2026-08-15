package books

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

// CheckResult holds books validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// CheckOptions controls books check behavior.
type CheckOptions struct {
	// IncludeHidden reports whether hidden (dot-prefixed) YAML files are checked.
	// By default hidden files are ignored.
	IncludeHidden bool
}

// RunCheck validates books YAML syntax and structure.
func RunCheck(path string) (*CheckResult, error) {
	return RunCheckWithOptions(path, CheckOptions{})
}

// RunCheckWithOptions validates books YAML against the embedded books JSON
// Schema. No post-rule is needed: the schema already enforces row name, score
// range, and enumerated row keys (additionalProperties false).
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	sch, err := schemacheck.CompileBytes(schema.Books)
	if err != nil {
		return nil, fmt.Errorf("compile books schema: %w", err)
	}

	files, err := fileutil.ListYAMLFilesRecursive(path, fileutil.ListOptions{IncludeHidden: opts.IncludeHidden})
	if err != nil {
		return nil, fmt.Errorf("list books yaml under %s: %w", path, err)
	}

	var issues []checkutil.Issue
	for _, file := range files {
		issues = append(issues, schemacheck.CheckFile(file, sch, nil)...)
	}

	return &CheckResult{Issues: issues}, nil
}
