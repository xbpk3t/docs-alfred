package books

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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
// Schema (shared with ntl). No post-rule is needed: the schema enforces row
// name, score range, enumerated row keys (additionalProperties false), and
// publishAt as a year integer.
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	issues, err := schemacheck.CheckDirectory(path, "books", schema.Books, opts.IncludeHidden)
	if err != nil {
		return nil, err
	}

	return &CheckResult{Issues: issues}, nil
}
