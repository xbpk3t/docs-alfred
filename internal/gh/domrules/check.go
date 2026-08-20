package domrules

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml/ast"
	yamlparser "github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/yamlutil"
)

const (
	fieldDes    = "des"
	fieldScore  = "score"
	fieldReadAt = "readAt"
	fieldRecord = "record"
	fieldSub    = "sub"
	fieldItem   = "item"
	fieldTags   = "tags"
	fieldRecite = "recite"
	fieldDate   = "date"
	fieldTable  = "table"
	fieldURL    = "url"
	extYML      = ".yml"
	extYAML     = ".yaml"
)

// CheckResult is the result of running a data check.
type CheckResult struct {
	Issues []checkutil.Issue
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
		docIssues := checkItemsAST(file, seq, allowedFields)
		issues = append(issues, docIssues...)
	}

	return issues
}

// ---- AST-based checking (with line/col) ----

func checkItemsAST(file string, seq *ast.SequenceNode, allowedFields map[string]bool) []checkutil.Issue {
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
		issues = append(issues, checkMappingAST(file, mapping, allowedFields)...)
	}

	return issues
}

func checkMappingAST(file string, mapping *ast.MappingNode, allowedFields map[string]bool) []checkutil.Issue {
	var issues []checkutil.Issue

	for _, kv := range mapping.Values {
		if kv == nil {
			continue
		}
		key := yamlutil.KeyString(kv.Key)
		if key == "" {
			continue
		}
		issues = append(issues, checkKeyValueAST(file, key, kv, allowedFields)...)
	}

	return issues
}
func checkKeyValueAST(file, key string, kv *ast.MappingValueNode, allowedFields map[string]bool) []checkutil.Issue {
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
	issues = append(issues, checkFieldValueAST(file, key, val)...)

	return issues
}
func checkFieldValueAST(file, key string, val ast.Node) []checkutil.Issue {
	switch key {
	case fieldScore:
		return checkScoreFieldAST(file, val)
	case fieldDate:
		return checkDateFieldValueAST(file, val, "date")
	case "endDate":
		return checkDateFieldValueAST(file, val, "endDate")
	case fieldReadAt:
		return checkDateFieldValueAST(file, val, "readAt")
	case fieldRecord:
		return checkIsSequenceAST(file, val, "record")
	case fieldSub:
		return checkSubFieldAST(file, val)
	case fieldItem:
		return checkIsSequenceAST(file, val, "item")
	case fieldTags:
		if _, ok := val.(*ast.SequenceNode); !ok {
			return []checkutil.Issue{warnIssue(file, val, "tags 建议使用数组")}
		}
	case fieldTable, fieldRecite:
		return checkIsSequenceAST(file, val, key)
	}

	return nil
}

func checkScoreFieldAST(file string, val ast.Node) []checkutil.Issue {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case *ast.IntegerNode:
		return checkIntScoreAST(file, v)
	case *ast.FloatNode:
		score := int(v.Value)
		if float64(score) != v.Value || !validScore(score) {
			return []checkutil.Issue{errIssue(file, val, "score 必须是整数且范围 0-5")}
		}
	case *ast.StringNode:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	default:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	}

	return nil
}

// validScore reports whether score is within the allowed 0-5 range.
func validScore(score int) bool {
	return score >= 0 && score <= 5
}

func checkIntScoreAST(file string, val *ast.IntegerNode) []checkutil.Issue {
	switch v := val.Value.(type) {
	case int64:
		score := int(v)
		if !validScore(score) {
			return []checkutil.Issue{errIssue(file, val, "score 范围必须是 0-5")}
		}
	case uint64:
		if v > 5 {
			return []checkutil.Issue{errIssue(file, val, "score 范围必须是 0-5")}
		}
	}

	return nil
}

func checkDateFieldValueAST(file string, val ast.Node, field string) []checkutil.Issue {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case *ast.StringNode:
		if !DateFull.MatchString(v.Value) {
			return []checkutil.Issue{errIssue(file, val, fmt.Sprintf("%s 必须是 YYYY-MM-DD 格式: %s", field, v.Value))}
		}
	default:
		return []checkutil.Issue{errIssue(file, val, field+" 必须是字符串")}
	}

	return nil
}

func checkIsSequenceAST(file string, val ast.Node, field string) []checkutil.Issue {
	if _, ok := val.(*ast.SequenceNode); !ok {
		return []checkutil.Issue{errIssue(file, val, field+" 必须是数组")}
	}

	return nil
}

func checkSubFieldAST(file string, val ast.Node) []checkutil.Issue {
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
		issues = append(issues, checkMappingAST(file, mapping, AllowedFieldsForScope(ScopeDiary))...)
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
