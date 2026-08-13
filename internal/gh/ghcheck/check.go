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
// enum itself lives in gh.schema.json.
const (
	KindMechanism = "mech"
	KindType      = "type"
	KindRepo      = "repo"
	KindTools     = "tools"
	KindTemp      = "temp"

	// MaxTopicsPerSection is the max topics allowed under one section type.
	MaxTopicsPerSection = 30

	// AllowedKindsCSV is the allowed set shown in error messages.
	AllowedKindsCSV = "mech|type|repo|tools|temp"
)

// CheckResult holds gh validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// RunCheck validates all YAML files under ghRoot against the embedded gh
// JSON Schema, plus the gh-only rule that every topic carries a kind. Missing
// path returns a Go error; per-file YAML/shape problems become Issues.
func RunCheck(ghRoot string) (*CheckResult, error) {
	sch, err := schemacheck.CompileBytes(schema.Gh)
	if err != nil {
		return nil, fmt.Errorf("compile gh schema: %w", err)
	}

	files, err := fileutil.ListYAMLFilesRecursive(ghRoot)
	if err != nil {
		return nil, fmt.Errorf("list gh yaml under %s: %w", ghRoot, err)
	}

	var issues []checkutil.Issue
	for _, file := range files {
		issues = append(issues, schemacheck.CheckFile(file, sch, requireTopicKinds)...)
	}

	return &CheckResult{Issues: issues}, nil
}

// requireTopicKinds is the gh-only business rule: every topic must carry a
// kind. The kind enum stays in the schema; this post-rule enforces presence so
// the schema itself stays shared with goods (which has no kind).
func requireTopicKinds(doc any, path string) []checkutil.Issue {
	var issues []checkutil.Issue
	var walkTopics func(topics any)
	walkTopics = func(topics any) {
		list, ok := topics.([]any)
		if !ok {
			return
		}
		for _, item := range list {
			topic, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := topic["kind"]; !ok {
				issues = append(issues, checkutil.Issue{
					File:     path,
					Severity: checkutil.SeverityError,
					Message:  fmt.Sprintf("topic %q 缺少必填字段 kind", topic["topic"]),
				})
			}
			// recurse into nested subtopics
			if nested, ok := topic["topics"]; ok {
				walkTopics(nested)
			}
		}
	}
	sections, ok := doc.([]any)
	if !ok {
		return issues
	}
	for _, s := range sections {
		section, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if topics, ok := section["topics"]; ok {
			walkTopics(topics)
		}
	}

	return issues
}
