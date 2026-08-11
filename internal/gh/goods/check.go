package goods

import (
	"fmt"

	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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

// RunCheckWithOptions validates goods YAML syntax and structure.
// It reuses the shared structured-check engine with the goods rule scope,
// so hidden .goods.*.yml files follow exactly the same rules as goods.*.yml.
func RunCheckWithOptions(path string, opts CheckOptions) (*CheckResult, error) {
	result, err := data.RunStructuredDataCheckWithOptions(path, string(data.ScopeGoods), data.RunStructuredCheckOptions{
		IncludeHidden: opts.IncludeHidden,
	})
	if err != nil {
		return nil, fmt.Errorf("goods check: %w", err)
	}

	return &CheckResult{Issues: result.Issues}, nil
}
