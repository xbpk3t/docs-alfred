package wiki

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

const codeFence = "```"

// canonicalSectionHeadings are the only allowed #### section headings in summary entries.
var canonicalSectionHeadings = map[string]bool{
	"概述":    true,
	"关键要点":  true,
	"可执行建议": true,
	"值得关注":  true,
}

// validCodeblockFields are the allowed fields in summary codeblocks.
var validCodeblockFields = map[string]bool{
	"URL":               true,
	"Type":              true,
	"tags":              true,
	"quality":           true,
	"author":            true,
	"uncertainties":     true,
	"duration":          true,
	"transcriptQuality": true,
	"verdict":           true,
	"stars":             true,
	"language":          true,
}

var sectionHeadingRe = regexp.MustCompile(`^####\s+(.+)$`)

// AuditWiki scans wiki markdown files for polluted successful entries and malformed URLs.
func AuditWiki(wikiRoot string) ([]checkutil.Issue, error) {
	var issues []checkutil.Issue
	err := filepath.WalkDir(wikiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, auditIssue(wikiRoot, path, 0, checkutil.SeverityError,
				fmt.Sprintf("walk error: %v", walkErr)))

			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		issues = append(issues, auditMarkdownFile(wikiRoot, path)...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return issues, nil
}

// AuditWikiPaths audits only the supplied wiki markdown files/directories.
func AuditWikiPaths(wikiRoot string, paths []string) ([]checkutil.Issue, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	var issues []checkutil.Issue
	seen := make(map[string]bool)
	for _, rawPath := range paths {
		pathIssues, err := auditWikiPath(wikiRoot, rawPath, seen)
		if err != nil {
			return nil, err
		}
		issues = append(issues, pathIssues...)
	}

	return issues, nil
}

func auditWikiPath(wikiRoot, rawPath string, seen map[string]bool) ([]checkutil.Issue, error) {
	path, err := resolveAuditPath(wikiRoot, rawPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return []checkutil.Issue{auditIssue(wikiRoot, path, 0, checkutil.SeverityError,
			fmt.Sprintf("stat error: %v", err))}, nil
	}
	if info.IsDir() {
		return auditWikiDir(wikiRoot, path, seen)
	}
	if filepath.Ext(path) != ".md" || seen[path] {
		return nil, nil
	}
	seen[path] = true

	return auditMarkdownFile(wikiRoot, path), nil
}

func auditWikiDir(wikiRoot, path string, seen map[string]bool) ([]checkutil.Issue, error) {
	var issues []checkutil.Issue
	walkErr := filepath.WalkDir(path, func(child string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, auditIssue(wikiRoot, child, 0, checkutil.SeverityError,
				fmt.Sprintf("walk error: %v", walkErr)))

			return nil
		}
		if d.IsDir() || filepath.Ext(child) != ".md" || seen[child] {
			return nil
		}
		seen[child] = true
		issues = append(issues, auditMarkdownFile(wikiRoot, child)...)

		return nil
	})

	return issues, walkErr
}

func auditMarkdownFile(wikiRoot, path string) []checkutil.Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []checkutil.Issue{auditIssue(wikiRoot, path, 0, checkutil.SeverityError,
			fmt.Sprintf("read error: %v", err))}
	}
	lines := strings.Split(string(data), "\n")
	rel := slashRel(wikiRoot, path)
	var issues []checkutil.Issue
	if filepath.Base(path) == "summary.md" {
		issues = append(issues, auditSummaryFile(rel, lines)...)
	}
	if isFailureFile(wikiRoot, path) || filepath.Base(path) == "inbox.md" {
		issues = append(issues, auditMalformedURLLines(rel, lines)...)
	}

	return issues
}

func resolveAuditPath(wikiRoot, rawPath string) (string, error) {
	path, err := auditPathCandidate(wikiRoot, rawPath)
	if err != nil {
		return "", err
	}
	absRoot, err := absEvalSymlink(wikiRoot)
	if err != nil {
		return "", err
	}
	absPath, err := absEvalSymlink(path)
	if err != nil {
		return "", err
	}
	withinRoot, err := pathWithinRoot(absRoot, absPath)
	if err != nil {
		return "", err
	}
	if !withinRoot {
		return "", fmt.Errorf("audit path outside wiki root: %s", rawPath)
	}

	return absPath, nil
}

func auditPathCandidate(wikiRoot, rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", errors.New("audit path is empty")
	}
	if !filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(wikiRoot, path)
		}
	}

	return path, nil
}

func absEvalSymlink(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, ok := evalSymlinks(absPath); ok {
		return resolved, nil
	}

	return absPath, nil
}

func evalSymlinks(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}

	return resolved, true
}

func pathWithinRoot(absRoot, absPath string) (bool, error) {
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false, err
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel), nil
}

func auditSummaryFile(file string, lines []string) []checkutil.Issue {
	issues := auditMalformedURLLines(file, lines)
	lowerAll := strings.ToLower(strings.Join(lines, "\n"))
	if matches := lowQualityMatcher.Match([]byte(lowerAll)); len(matches) > 0 {
		issues = append(issues, checkutil.Issue{
			File:     file,
			Severity: checkutil.SeverityError,
			Message:  "successful summary contains low-quality extraction marker: " + lowQualityPatternsList[matches[0]],
		})
	}
	issues = append(issues, auditCanonicalHeadings(file, lines)...)
	issues = append(issues, auditCodeblockFields(file, lines)...)

	return issues
}

func auditCanonicalHeadings(file string, lines []string) []checkutil.Issue {
	var issues []checkutil.Issue
	for i, line := range lines {
		m := sectionHeadingRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		heading := m[1]
		if !canonicalSectionHeadings[heading] {
			issues = append(issues, checkutil.Issue{
				File:     file,
				Line:     i + 1,
				Severity: checkutil.SeverityWarn,
				Message:  fmt.Sprintf("non-canonical section heading: #### %s (allowed: 概述, 关键要点, 可执行建议, 值得关注)", heading),
			})
		}
	}

	return issues
}

func auditCodeblockFields(file string, lines []string) []checkutil.Issue {
	var issues []checkutil.Issue
	inCodeblock := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == codeFence || strings.HasPrefix(trimmed, codeFence) {
			inCodeblock = !inCodeblock

			continue
		}
		if !inCodeblock {
			continue
		}
		// Inside a codeblock — check key: value pairs.
		fieldName, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		fieldName = strings.TrimSpace(fieldName)
		if fieldName != "" && !validCodeblockFields[fieldName] {
			issues = append(issues, checkutil.Issue{
				File:     file,
				Line:     i + 1,
				Severity: checkutil.SeverityWarn,
				Message:  "unknown codeblock field: " + fieldName,
			})
		}
	}

	return issues
}

func auditMalformedURLLines(file string, lines []string) []checkutil.Issue {
	var issues []checkutil.Issue
	for i, line := range lines {
		if lineHasRawMalformedURL(line) {
			issues = append(issues, checkutil.Issue{
				File:     file,
				Line:     i + 1,
				Severity: checkutil.SeverityError,
				Message:  "malformed URL contains markdown link syntax",
			})
		}
	}

	return issues
}

func lineHasRawMalformedURL(line string) bool {
	before0, _, ok := strings.Cut(line, "](")
	if !ok {
		return false
	}
	before := before0
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "## [") && !strings.Contains(before, "- URL:") {
		return false
	}
	if strings.LastIndex(before, "[") >= 0 {
		return false
	}

	lastTokenStart := strings.LastIndexAny(before, " \t([")
	lastToken := before[lastTokenStart+1:]

	return strings.HasPrefix(lastToken, "http://") || strings.HasPrefix(lastToken, "https://")
}

func isFailureFile(wikiRoot, path string) bool {
	rel := slashRel(wikiRoot, path)

	return strings.HasSuffix(rel, "-failed.md") || strings.HasPrefix(filepath.Base(rel), "digest-")
}

func auditIssue(wikiRoot, path string, line int, severity, message string) checkutil.Issue {
	return checkutil.Issue{File: slashRel(wikiRoot, path), Line: line, Severity: severity, Message: message}
}

func slashRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}
