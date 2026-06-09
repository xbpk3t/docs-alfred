package goods

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/goccy/go-yaml/ast"
	yamlparser "github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/yamlutil"
	"github.com/xbpk3t/docs-alfred/service/data"
)

const (
	fieldDate     = "date"
	fieldEndDate  = "endDate"
	fieldEndPrice = "endPrice"
	fieldItem     = "item"
	fieldTag      = "tag"
	fieldUsing    = "using"
)

var (
	eligibleLifecycleTags = map[string]bool{
		"EDC":     true,
		"bedding": true,
		"clothes": true,
		"电子设备":    true,
		"耐用品":     true,
	}
	strictCNYPricePattern = regexp.MustCompile(`^(?:[¥￥]\s*)?\d+(?:\.\d+)?$`)
)

// CheckResult holds goods validation issues.
type CheckResult struct {
	Issues []checkutil.Issue
}

// RunCheck validates goods YAML syntax and lifecycle-cost fields.
func RunCheck(path string) (*CheckResult, error) {
	if _, errs := data.ParseYAMLDir(path); len(errs) > 0 {
		for _, err := range errs {
			return nil, fmt.Errorf("goods YAML parse: %w", err)
		}
	}

	files, err := fileutil.ListYAMLFiles(path)
	if err != nil {
		return nil, err
	}

	var issues []checkutil.Issue
	for _, file := range files {
		issues = append(issues, checkFile(file)...)
	}

	return &CheckResult{Issues: issues}, nil
}

func checkFile(file string) []checkutil.Issue {
	raw, err := os.ReadFile(file)
	if err != nil {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("read error: %v", err)}}
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}

	parsed, err := yamlparser.ParseBytes(raw, yamlparser.ParseComments)
	if err != nil {
		return []checkutil.Issue{{File: file, Severity: checkutil.SeverityError, Message: fmt.Sprintf("YAML parse error: %v", err)}}
	}

	var issues []checkutil.Issue
	for _, doc := range parsed.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		seq, ok := yamlutil.Sequence(doc.Body)
		if !ok {
			issues = append(issues, errorIssue(file, doc.Body, "顶层必须是列表"))

			continue
		}
		issues = append(issues, checkCategories(file, seq)...)
	}

	return issues
}

func checkCategories(file string, seq *ast.SequenceNode) []checkutil.Issue {
	var issues []checkutil.Issue
	for _, item := range seq.Values {
		category, ok := yamlutil.Mapping(item)
		if !ok {
			issues = append(issues, errorIssue(file, item, "goods 项必须是对象"))

			continue
		}

		tag, _ := nodeString(yamlutil.MappingValue(category, fieldTag))
		eligible := eligibleLifecycleTags[tag]
		issues = append(issues, checkCategoryLifecycleFields(file, category)...)
		issues = append(issues, checkLifecycleMap(file, tag, eligible, yamlutil.MappingValue(category, fieldUsing))...)
		issues = append(issues, checkLifecycleItems(file, tag, eligible, yamlutil.MappingValue(category, fieldItem))...)
	}

	return issues
}

func checkCategoryLifecycleFields(file string, category *ast.MappingNode) []checkutil.Issue {
	var issues []checkutil.Issue
	if endDateNode := yamlutil.MappingValue(category, fieldEndDate); endDateNode != nil {
		issues = append(issues, errorIssue(file, endDateNode, "endDate 只能写在 using 或 item[] 项上"))
	}
	if endPriceNode := yamlutil.MappingValue(category, fieldEndPrice); endPriceNode != nil {
		issues = append(issues, errorIssue(file, endPriceNode, "endPrice 只能写在 using 或 item[] 项上"))
	}

	return issues
}

func checkLifecycleItems(file, tag string, eligible bool, itemNode ast.Node) []checkutil.Issue {
	if itemNode == nil {
		return nil
	}
	seq, ok := yamlutil.Sequence(itemNode)
	if !ok {
		return nil
	}

	var issues []checkutil.Issue
	for _, rawItem := range seq.Values {
		issues = append(issues, checkLifecycleMap(file, tag, eligible, rawItem)...)
	}

	return issues
}

func checkLifecycleMap(file, tag string, eligible bool, rawItem ast.Node) []checkutil.Issue {
	item, ok := yamlutil.Mapping(rawItem)
	if !ok || item == nil {
		return nil
	}

	endDateNode := yamlutil.MappingValue(item, fieldEndDate)
	endPriceNode := yamlutil.MappingValue(item, fieldEndPrice)
	if endDateNode == nil && endPriceNode == nil {
		return nil
	}

	var issues []checkutil.Issue
	issues = append(issues, checkLifecycleEligibility(file, tag, eligible, endDateNode, endPriceNode)...)
	issues = append(issues, checkLifecycleDates(file, item, endDateNode)...)
	issues = append(issues, checkLifecycleEndPrice(file, endDateNode, endPriceNode)...)

	return issues
}

func checkLifecycleEligibility(file, tag string, eligible bool, endDateNode, endPriceNode ast.Node) []checkutil.Issue {
	if eligible {
		return nil
	}

	var issues []checkutil.Issue
	if endDateNode != nil {
		issues = append(issues, errorIssue(file, endDateNode, fmt.Sprintf("endDate 只允许用于生命周期实物 tag，当前 tag=%q", tag)))
	}
	if endPriceNode != nil {
		issues = append(issues, errorIssue(file, endPriceNode, fmt.Sprintf("endPrice 只允许用于生命周期实物 tag，当前 tag=%q", tag)))
	}

	return issues
}

func checkLifecycleDates(file string, item *ast.MappingNode, endDateNode ast.Node) []checkutil.Issue {
	date, hasDate, issues := parseRequiredDate(file, item, fieldDate, endDateNode)
	endDate, hasEndDate, endDateIssues := parseOptionalDate(file, endDateNode, fieldEndDate)
	issues = append(issues, endDateIssues...)
	if hasDate && hasEndDate && endDate.Before(date) {
		issues = append(issues, errorIssue(file, endDateNode, "endDate 不能早于 date"))
	}

	return issues
}

func checkLifecycleEndPrice(file string, endDateNode, endPriceNode ast.Node) []checkutil.Issue {
	if endPriceNode == nil {
		return nil
	}

	var issues []checkutil.Issue
	if endDateNode == nil {
		issues = append(issues, errorIssue(file, endPriceNode, "endPrice 必须和 endDate 同时存在"))
	}
	if endPrice, ok := nodeString(endPriceNode); !ok || !isStrictCNYPrice(endPrice) {
		issues = append(issues, errorIssue(file, endPriceNode, "endPrice 必须是明确的一次性人民币金额"))
	}

	return issues
}

func parseRequiredDate(file string, item *ast.MappingNode, field string, anchor ast.Node) (time.Time, bool, []checkutil.Issue) {
	node := yamlutil.MappingValue(item, field)
	if node == nil {
		return time.Time{}, false, []checkutil.Issue{errorIssue(file, anchor, "endDate 必须有同级 date")}
	}

	return parseOptionalDate(file, node, field)
}

func parseOptionalDate(file string, node ast.Node, field string) (time.Time, bool, []checkutil.Issue) {
	if node == nil {
		return time.Time{}, false, nil
	}
	value, ok := nodeString(node)
	if !ok || !checkutil.DateFullPattern.MatchString(value) {
		return time.Time{}, false, []checkutil.Issue{errorIssue(file, node, field+" 必须是 YYYY-MM-DD 格式")}
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, false, []checkutil.Issue{errorIssue(file, node, field+" 不是有效日期")}
	}

	return parsed, true, nil
}

func nodeString(node ast.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	switch v := node.(type) {
	case *ast.StringNode:
		return strings.TrimSpace(v.Value), true
	case *ast.IntegerNode:
		return strings.TrimSpace(v.String()), true
	case *ast.FloatNode:
		return strings.TrimSpace(v.String()), true
	default:
		return "", false
	}
}

func isStrictCNYPrice(value string) bool {
	return strictCNYPricePattern.MatchString(strings.TrimSpace(value))
}

func errorIssue(file string, node ast.Node, message string) checkutil.Issue {
	return checkutil.Issue{File: file, Line: yamlutil.NodeLine(node), Severity: checkutil.SeverityError, Message: message}
}
