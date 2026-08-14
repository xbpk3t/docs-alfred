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
		// deliberately not wired here. goodsPostRule adds the goods-only table
		// rules that the free-form tableItem schema cannot express.
		issues = append(issues, schemacheck.CheckFile(file, sch, goodsPostRule)...)
	}

	return &CheckResult{Issues: issues}, nil
}

// goodsPostRule enforces the goods-only business rules that the JSON Schema
// cannot express (tableItem is free-form):
//   - table rows must carry a name
//   - endPrice requires endDate (a sale price without a disposal date is
//     meaningless; endDate alone is allowed, e.g. disposed without a price)
//   - endDate/endPrice are only valid inside table rows, never on a
//     section/topic
func goodsPostRule(doc any, path string) []checkutil.Issue {
	var issues []checkutil.Issue
	sections, ok := doc.([]any)
	if !ok {
		return nil
	}
	for _, s := range sections {
		section, ok := s.(map[string]any)
		if !ok {
			continue
		}
		issues = append(issues, checkNoTableOnlyField(section, path, "section")...)
		topics, _ := section["topics"].([]any)
		for _, t := range topics {
			topic, ok := t.(map[string]any)
			if !ok {
				continue
			}
			issues = append(issues, checkNoTableOnlyField(topic, path, "topic")...)
			table, _ := topic["table"].([]any)
			for _, row := range table {
				rowMap, ok := row.(map[string]any)
				if !ok {
					continue
				}
				issues = append(issues, checkTableRowRules(rowMap, path)...)
			}
		}
	}

	return issues
}

// tableOnlyFields may only appear inside table rows (goods items).
var tableOnlyFields = []string{"endDate", "endPrice"}

func checkNoTableOnlyField(m map[string]any, path, where string) []checkutil.Issue {
	var issues []checkutil.Issue
	for _, f := range tableOnlyFields {
		if _, ok := m[f]; ok {
			issues = append(issues, checkutil.Issue{
				File:     path,
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("%s 只能写在 table 项上 (%s)", f, where),
			})
		}
	}

	return issues
}

func checkTableRowRules(row map[string]any, path string) []checkutil.Issue {
	var issues []checkutil.Issue
	if _, ok := row["name"]; !ok {
		issues = append(issues, checkutil.Issue{
			File:     path,
			Severity: checkutil.SeverityError,
			Message:  "table 项缺少必填字段 name",
		})
	}
	if _, hasPrice := row["endPrice"]; hasPrice {
		if _, hasDate := row["endDate"]; !hasDate {
			issues = append(issues, checkutil.Issue{
				File:     path,
				Severity: checkutil.SeverityError,
				Message:  "endPrice 必须和 endDate 同时存在",
			})
		}
	}

	return issues
}
