package data

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	yamlparser "github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
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
	files, err := listYAMLFiles(targetDir)
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

func listYAMLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == extYML || ext == extYAML {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	return files, nil
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
				File: file, Line: nodeLine(doc.Body),
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

// nodeLine safely extracts the line number from an AST node.
func nodeLine(n ast.Node) int {
	if n == nil {
		return 0
	}
	tk := n.GetToken()
	if tk == nil {
		return 0
	}

	return tk.Position.Line
}

func checkItemsAST(file string, seq *ast.SequenceNode, allowedFields map[string]bool, scope RuleScope) []checkutil.Issue {
	var issues []checkutil.Issue
	for i, item := range seq.Values {
		mapping, ok := item.(*ast.MappingNode)
		if !ok {
			issues = append(issues, checkutil.Issue{
				File: file, Line: nodeLine(item),
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

	for _, kv := range mapping.Values {
		if kv == nil {
			continue
		}
		key := keyString(kv.Key)
		if key == "" {
			continue
		}
		if key == fieldName {
			hasName = true
		}
		issues = append(issues, checkKeyValueAST(file, key, kv, allowedFields, scope)...)
	}

	// Check required fields
	if scope != ScopeDiary && scope != ScopeJav {
		if !hasName {
			issues = append(issues, checkutil.Issue{
				File: file, Line: nodeLine(mapping),
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("缺少必填字段 name (%s)", path),
			})
		}
	}

	return issues
}

// keyString extracts the string value from a MapKeyNode.
func keyString(n ast.MapKeyNode) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.StringNode:
		return v.Value
	case *ast.IntegerNode:
		if val, ok := v.Value.(int64); ok {
			return strconv.FormatInt(val, 10)
		}

		return fmt.Sprintf("%v", v.Value)
	}

	return n.String()
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
	if isNullValue(val) && key != fieldDes {
		issues = append(issues, warnIssue(file, val, fmt.Sprintf("字段 %s 为空，建议省略", key)))
	}

	// Field-specific type/value checks
	issues = append(issues, checkFieldValueAST(file, key, val, scope)...)

	return issues
}

// isNullValue checks if an AST node represents a null/empty value.
func isNullValue(n ast.Node) bool {
	if n == nil {
		return true
	}
	// Check if it's a null node
	if _, ok := n.(*ast.NullNode); ok {
		return true
	}
	// Check if it's an empty string
	if s, ok := n.(*ast.StringNode); ok && strings.TrimSpace(s.Value) == "" {
		return true
	}

	return false
}

func checkFieldValueAST(file, key string, val ast.Node, scope RuleScope) []checkutil.Issue {
	switch key {
	case fieldScore:
		return checkScoreFieldAST(file, val)
	case fieldReadAt:
		return checkDateFieldValueAST(file, val, "readAt", DateFull, "date")
	case fieldPublishAt:
		return checkPublishAtAST(file, val, scope)
	case fieldRecord:
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
		if float64(score) != v.Value || score < 0 || score > 5 {
			return []checkutil.Issue{errIssue(file, val, "score 必须是整数且范围 0-5")}
		}
	case *ast.StringNode:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	default:
		return []checkutil.Issue{errIssue(file, val, "score 必须是整数")}
	}

	return nil
}

func checkIntScoreAST(file string, val *ast.IntegerNode) []checkutil.Issue {
	switch v := val.Value.(type) {
	case int64:
		score := int(v)
		if score < 0 || score > 5 {
			return []checkutil.Issue{errIssue(file, val, "score 范围必须是 0-5")}
		}
	case uint64:
		score := int(v)
		if score < 0 || score > 5 {
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
	return checkutil.Issue{File: file, Line: nodeLine(n), Severity: checkutil.SeverityError, Message: msg}
}

func warnIssue(file string, n ast.Node, msg string) checkutil.Issue {
	return checkutil.Issue{File: file, Line: nodeLine(n), Severity: checkutil.SeverityWarn, Message: msg}
}

// ---- decoder-based fallback (used by tests) ----.
func checkScoreField(file string, val any) []checkutil.Issue {
	switch v := val.(type) {
	case int:
		if v < 0 || v > 5 {
			return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: "score 范围必须是 0-5"}}
		}
	case float64:
		score := int(v)
		if float64(score) != v || score < 0 || score > 5 {
			return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: "score 必须是整数且范围 0-5"}}
		}
	default:
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: "score 必须是整数"}}
	}

	return nil
}

func checkDateFieldValue(file string, val any, field string, pattern *regexp.Regexp, kind string) []checkutil.Issue {
	str, ok := val.(string)
	if !ok {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: field + " 必须是字符串"}}
	}

	if kind == fieldDate && !pattern.MatchString(str) {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("%s 必须是 YYYY-MM-DD 格式: %s", field, str)}}
	}
	if kind == kindYear && !pattern.MatchString(str) {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("%s 格式错误: %s", field, str)}}
	}

	return nil
}

func checkIsSequence(file string, val any, field string) []checkutil.Issue {
	if _, ok := val.([]any); !ok {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: field + " 必须是数组"}}
	}

	return nil
}
