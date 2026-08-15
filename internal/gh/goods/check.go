package goods

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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
// Schema. No post-rule is needed: the schema enumerates all goods row keys
// (additionalProperties false), requires name, and expresses the
// endPrice → endDate relationship via dependentRequired.
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	issues, err := schemacheck.CheckDirectory(path, "goods", schema.Goods, opts.IncludeHidden)
	if err != nil {
		return nil, err
	}

	return &CheckResult{Issues: issues}, nil
}
