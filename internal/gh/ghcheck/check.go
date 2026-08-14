// Package ghcheck validates data/gh YAML against the embedded gh JSON Schema.
package ghcheck

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

// Allowed topic.kind values for data/gh. Shared with index.TopicCatalog; the
// enum and required-kind themselves live in gh.schema.json.
const (
	KindMechanism = "mech"
	KindType      = "type"
	KindRepo      = "repo"
	KindTools     = "tools"
	KindTemp      = "temp"

	// MaxTopicsPerSection is the max topics allowed under one section type.
	MaxTopicsPerSection = 30
)

// CheckResult holds gh validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// CheckOptions controls gh check behavior.
type CheckOptions struct {
	// IncludeHidden reports whether hidden (dot-prefixed) YAML files are checked.
	// By default hidden files are ignored.
	IncludeHidden bool
}

// RunCheck validates all YAML files under ghRoot against the embedded gh
// JSON Schema. Missing path returns a Go error; per-file YAML/shape problems
// become Issues.
func RunCheck(ghRoot string) (*CheckResult, error) {
	return RunCheckWithOptions(ghRoot, CheckOptions{})
}

// RunCheckWithOptions validates gh YAML against the embedded gh JSON Schema,
// optionally including hidden (dot-prefixed) files.
func RunCheckWithOptions(ghRoot string, opts CheckOptions) (*CheckResult, error) {
	sch, err := schemacheck.CompileBytes(schema.Gh)
	if err != nil {
		return nil, fmt.Errorf("compile gh schema: %w", err)
	}

	files, err := fileutil.ListYAMLFilesRecursive(ghRoot, fileutil.ListOptions{IncludeHidden: opts.IncludeHidden})
	if err != nil {
		return nil, fmt.Errorf("list gh yaml under %s: %w", ghRoot, err)
	}

	var issues []checkutil.Issue
	for _, file := range files {
		issues = append(issues, schemacheck.CheckFile(file, sch, nil)...)
	}

	return &CheckResult{Issues: issues}, nil
}
