package domrules

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	yamlparser "github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/yamlutil"
)

const (
	fieldDes       = "des"
	fieldScore     = "score"
	fieldReadAt    = "readAt"
	fieldPublishAt = "publishAt"
	fieldRecord    = "record"
	fieldSub       = "sub"
	fieldItem      = "item"
	fieldTags      = "tags"
	fieldRecite    = "recite"
	fieldDate      = "date"
	fieldTable     = "table"
	fieldName      = "name"
	fieldType      = "type"
	fieldURL       = "url"
	extYML         = ".yml"
	extYAML        = ".yaml"
	kindYear       = "year"
)

// CheckResult is the result of running a data check.
type CheckResult struct {
	Issues []checkutil.Issue
}

// RunStructuredDataCheck validates all YAML files in a directory against domain rules.
func RunStructuredDataCheck(targetDir, scope string) (*CheckResult, error) {
	return RunStructuredDataCheckWithOptions(targetDir, scope, RunStructuredCheckOptions{})
}

// RunStructuredCheckOptions controls structured check behavior.
type RunStructuredCheckOptions struct {
	// IncludeHidden reports whether hidden (dot-prefixed) YAML files are checked.
	IncludeHidden bool
}

// RunStructuredDataCheckWithOptions validates YAML files in a directory against domain rules.
func RunStructuredDataCheckWithOptions(targetDir, scope string, opts RunStructuredCheckOptions) (*CheckResult, error) {
	files, err := listYAMLFiles(targetDir, opts.IncludeHidden)
	if err != nil {
		return nil, err
	}

	var issues []checkutil.Issue
	for _, file := range files {
		fileIssues := checkFile(file, scope)
		issues = append(issues, fileIssues...)
	}

	return &CheckResult{Issues: issues}, nil
}

func listYAMLFiles(dir string, includeHidden ...bool) ([]string, error) {
	var opts []fileutil.ListOptions
	if len(includeHidden) > 0 && includeHidden[0] {
		opts = append(opts, fileutil.ListOptions{IncludeHidden: true})
	}

	return fileutil.ListYAMLFiles(dir, opts...)
}

func checkFile(file, scope string) []checkutil.Issue {
	data, err := os.ReadFile(file)
	if err != nil {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("read error: %v", err)}}
	}

	if strings.TrimSpace(string(data)) == "" {
		return nil
	}

	ruleScope := ResolveScope(file, scope)
	allowedFields := AllowedFieldsForScope(ruleScope)
	var issues []checkutil.Issue

	// Parse multi-document YAML via AST (preserves line/col positions)
	parsed, err := yamlparser.ParseBytes(data, yamlparser.ParseComments)
	if err != nil {
		return []checkutil.Issue{
			{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("YAML parse error: %v", err)},
		}
	}

	for _, doc := range parsed.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		seq, ok := doc.Body.(*ast.SequenceNode)
		if !ok {
			issues = append(issues, checkutil.Issue{
				File: file, Line: yamlutil.NodeLine(doc.Body),
				Severity: checkutil.SeverityError, Message: "顶层必须是列表",
			})

			continue
		}
		docIssues := checkItemsAST(file, seq, allowedFields, ruleScope)
		issues = append(issues, docIssues...)
	}

	return issues
}

// ---- AST-based checking (with line/col) ----

func checkItemsAST(file string, seq *ast.SequenceNode, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	var issues []checkutil.Issue
	for i, item := range seq.Values {
		mapping, ok := item.(*ast.MappingNode)
		if !ok {
			issues = append(issues, checkutil.Issue{
				File: file, Line: yamlutil.NodeLine(item),
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("第 %d 项必须是对象", i+1),
			})

			continue
		}
		issues = append(issues, checkMappingAST(file, mapping, allowedFields, scope, fmt.Sprintf("[%d]", i+1))...)
	}

	return issues
}

func checkMappingAST(file string, mapping *ast.MappingNode, allowedFields map[string]bool, scope RuleScope, path string) []checkutil.Issue {
	var issues []checkutil.Issue
	hasName := false
	hasType := false

	for _, kv := range mapping.Values {
		if kv == nil {
			continue
		}
		key := yamlutil.KeyString(kv.Key)
		if key == "" {
			continue
		}
		if key == fieldName {
			hasName = true
		}
		if key == fieldType {
			hasType = true
		}
		issues = append(issues, checkKeyValueAST(file, key, kv, allowedFields, scope)...)
		if scope == ScopeGoods {
			issues = append(issues, checkNestedFieldAST(file, key, kv.Value, allowedFields, scope)...)
			if key == "using" {
				issues = append(issues, checkUsingFieldAST(file, kv.Value, allowedFields, scope)...)
			}
		}
	}

	// Check required fields
	if scope != ScopeDiary && scope != ScopeJav {
		required := "name"
		if scope == ScopeGoods {
			required = "type"
		}
		if (required == "type" && !hasType) || (required == "name" && !hasName) {
			issues = append(issues, checkutil.Issue{
				File: file, Line: yamlutil.NodeLine(mapping),
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("缺少必填字段 %s (%s)", required, path),
			})
		}
	}

	return issues
}

// checkNestedFieldAST descends into nested list fields and validates their items.
// It validates field names and value types (score/date/sequence) without
// requiring per-item mandatory fields, since nested items (topics, table rows)
// have their own shapes. checkKeyValueAST performs the undefined-field check,
// so no duplicate checks here.
func checkNestedFieldAST(file, key string, val ast.Node, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	seq, ok := yamlutil.Sequence(val)
	if !ok {
		return nil
	}

	var issues []checkutil.Issue
	for _, item := range seq.Values {
		mapping, ok := yamlutil.Mapping(item)
		if !ok {
			continue
		}
		for _, kv := range mapping.Values {
			if kv == nil {
				continue
			}
			childKey := yamlutil.KeyString(kv.Key)
			if childKey == "" {
				continue
			}
			issues = append(issues, checkKeyValueAST(file, childKey, kv, allowedFields, scope)...)
			if childKey == "using" {
				issues = append(issues, checkUsingFieldAST(file, kv.Value, allowedFields, scope)...)
			}
		}
	}

	return issues
}
func checkKeyValueAST(file, key string, kv *ast.MappingValueNode, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	var issues []checkutil.Issue

	if ForbiddenFields[key] {
		return append(issues, errIssue(file, kv.Key, "禁止字段: "+key))
	}

	if !allowedFields[key] {
		issues = append(issues, warnIssue(file, kv.Key, "未在规则中定义的字段: "+key))
	}

	val := kv.Value

	// Check null/empty values
	if yamlutil.IsNullOrEmptyString(val) && key != fieldDes {
		issues = append(issues, warnIssue(file, val, fmt.Sprintf("字段 %s 为空，建议省略", key)))
	}

	// Field-specific type/value checks
	issues = append(issues, checkFieldValueAST(file, key, val, allowedFields, scope)...)

	return issues
}
func checkFieldValueAST(file, key string, val ast.Node, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	switch key {
	case fieldScore:
		return checkScoreFieldAST(file, val, scope)
	case fieldDate:
		return checkDateFieldValueAST(file, val, "date", DateFull, "date")
	case fieldReadAt:
		return checkDateFieldValueAST(file, val, "readAt", DateFull, "date")
	case fieldPublishAt:
		return checkPublishAtAST(file, val, scope)
	case fieldRecord:
		if scope == ScopeGoods {
			return checkRecordFieldAST(file, val, allowedFields, scope)
		}

		return checkIsSequenceAST(file, val, "record")
	case fieldSub:
		return checkSubFieldAST(file, val, scope)
	case fieldItem:
		return checkIsSequenceAST(file, val, "item")
	case fieldTags:
		if _, ok := val.(*ast.SequenceNode); !ok {
			return []checkutil.Issue{warnIssue(file, val, "tags 建议使用数组")}
		}
	case fieldTable, fieldRecite:
		// Only goods tables follow a fixed item field set; other domains
		// (books etc.) use free-form Chinese table columns.
		if scope != ScopeGoods {
			return checkIsSequenceAST(file, val, key)
		}

		return checkTableFieldAST(file, val, allowedFields, scope, key)
	}

	return nil
}

// checkUsingFieldAST validates a goods using single-object mapping
// (name/date/price/...) by descending into its fields.
func checkUsingFieldAST(file string, val ast.Node, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	mapping, ok := yamlutil.Mapping(val)
	if !ok || mapping == nil {
		return nil
	}

	var issues []checkutil.Issue
	for _, kv := range mapping.Values {
		if kv == nil {
			continue
		}
		childKey := yamlutil.KeyString(kv.Key)
		if childKey == "" {
			continue
		}
		issues = append(issues, checkKeyValueAST(file, childKey, kv, allowedFields, scope)...)
	}

	return issues
}

// checkRecordFieldAST validates a goods record sequence: it must be an array
// of mappings (date/des/...), not plain strings.
func checkRecordFieldAST(file string, val ast.Node, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	seq, ok := yamlutil.Sequence(val)
	if !ok {
		return []checkutil.Issue{errIssue(file, val, "record 必须是数组")}
	}

	var issues []checkutil.Issue
	for _, item := range seq.Values {
		mapping, ok := yamlutil.Mapping(item)
		if !ok {
			issues = append(issues, errIssue(file, item, "record 项必须是对象（date/des/...）"))
			continue
		}
		for _, kv := range mapping.Values {
			if kv == nil {
				continue
			}
			childKey := yamlutil.KeyString(kv.Key)
			if childKey == "" {
				continue
			}
			issues = append(issues, checkKeyValueAST(file, childKey, kv, allowedFields, scope)...)
		}
	}

	return issues
}

// checkTableFieldAST validates a table/recite sequence and its item mappings.
func checkTableFieldAST(file string, val ast.Node, allowedFields map[string]bool, scope RuleScope, field string) []checkutil.Issue {
	seq, ok := yamlutil.Sequence(val)
	if !ok {
		return []checkutil.Issue{errIssue(file, val, field+" 必须是数组")}
	}

	var issues []checkutil.Issue
	for _, item := range seq.Values {
		mapping, ok := yamlutil.Mapping(item)
		if !ok {
			continue
		}
		for _, kv := range mapping.Values {
			if kv == nil {
				continue
			}
			childKey := yamlutil.KeyString(kv.Key)
			if childKey == "" {
				continue
			}
			issues = append(issues, checkKeyValueAST(file, childKey, kv, allowedFields, scope)...)
		}
	}

	return issues
}

func checkScoreFieldAST(file string, val ast.Node, scope RuleScope) []checkutil.Issue {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case *ast.IntegerNode:
		return checkIntScoreAST(file, v, scope)
	case *ast.FloatNode:
		score := int(v.Value)
		if float64(score) != v.Value || !validScore(score, scope) {
			return []checkutil.Issue{errIssue(file, val, "score 必须是整数且范围 0-5")}
		}
	case *ast.StringNode:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	default:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	}

	return nil
}

// validScore reports whether score is within the allowed range for a scope.
// goods allows -1 (no rating placeholder); other scopes require 0-5.
func validScore(score int, scope RuleScope) bool {
	if scope == ScopeGoods && score == -1 {
		return true
	}

	return score >= 0 && score <= 5
}

func checkIntScoreAST(file string, val *ast.IntegerNode, scope RuleScope) []checkutil.Issue {
	switch v := val.Value.(type) {
	case int64:
		score := int(v)
		if !validScore(score, scope) {
			return []checkutil.Issue{errIssue(file, val, "score 范围必须是 0-5")}
		}
	case uint64:
		if v > 5 {
			return []checkutil.Issue{errIssue(file, val, "score 范围必须是 0-5")}
		}
	}

	return nil
}

func checkDateFieldValueAST(file string, val ast.Node, field string, pattern *regexp.Regexp, kind string) []checkutil.Issue {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case *ast.StringNode:
		str := v.Value
		if kind == fieldDate && !pattern.MatchString(str) {
			return []checkutil.Issue{errIssue(file, val, fmt.Sprintf("%s 必须是 YYYY-MM-DD 格式: %s", field, str))}
		}
		if kind == kindYear && !pattern.MatchString(str) {
			return []checkutil.Issue{errIssue(file, val, fmt.Sprintf("%s 格式错误: %s", field, str))}
		}
	case *ast.IntegerNode:
		return checkDateFieldIntValueAST(file, v, field, pattern, kind)
	default:
		return []checkutil.Issue{errIssue(file, val, field+" 必须是字符串")}
	}

	return nil
}

func checkDateFieldIntValueAST(file string, val *ast.IntegerNode, field string, pattern *regexp.Regexp, kind string) []checkutil.Issue {
	if kind == fieldDate {
		return []checkutil.Issue{errIssue(file, val, field+" 必须是字符串")}
	}
	var yearStr string
	switch v := val.Value.(type) {
	case int64:
		yearStr = strconv.FormatInt(v, 10)
	case uint64:
		yearStr = strconv.FormatUint(v, 10)
	default:
		return []checkutil.Issue{errIssue(file, val, field+" 必须是字符串")}
	}
	if !pattern.MatchString(yearStr) {
		return []checkutil.Issue{errIssue(file, val, fmt.Sprintf("%s 格式错误: %s", field, yearStr))}
	}

	return nil
}

func checkPublishAtAST(file string, val ast.Node, scope RuleScope) []checkutil.Issue {
	switch scope {
	case ScopeBooks, ScopeMovie, ScopeJav, ScopeVG:
		return checkDateFieldValueAST(file, val, "publishAt", DateYear, kindYear)
	}

	return nil
}

func checkIsSequenceAST(file string, val ast.Node, field string) []checkutil.Issue {
	if _, ok := val.(*ast.SequenceNode); !ok {
		return []checkutil.Issue{errIssue(file, val, field+" 必须是数组")}
	}

	return nil
}

func checkSubFieldAST(file string, val ast.Node, scope RuleScope) []checkutil.Issue {
	seq, ok := val.(*ast.SequenceNode)
	if !ok {
		return []checkutil.Issue{errIssue(file, val, "sub 必须是数组")}
	}

	var issues []checkutil.Issue
	for _, item := range seq.Values {
		mapping, ok := item.(*ast.MappingNode)
		if !ok {
			issues = append(issues, errIssue(file, item, "sub 项必须是对象"))

			continue
		}
		issues = append(issues, checkMappingAST(file, mapping, AllowedFieldsForScope(scope), scope, "sub")...)
	}

	return issues
}

// ---- Issue helpers ----

func errIssue(file string, n ast.Node, msg string) checkutil.Issue {
	return checkutil.Issue{File: file, Line: yamlutil.NodeLine(n), Severity: checkutil.SeverityError, Message: msg}
}

func warnIssue(file string, n ast.Node, msg string) checkutil.Issue {
	return checkutil.Issue{File: file, Line: yamlutil.NodeLine(n), Severity: checkutil.SeverityWarn, Message: msg}
}
