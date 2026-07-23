package workspaceops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// wikiFrontmatter represents the OKF v0.1 frontmatter fields expected in wiki .md files.
// Session, Model, Issue, and Score are optional fields written by ccx session export
// (or edited manually); they are parsed for forward-compat but are never required.
type wikiFrontmatter struct {
	Title   string `yaml:"title"`
	Date    string `yaml:"date"`
	Source  string `yaml:"source"`
	Type    string `yaml:"type"`
	Session string `yaml:"session"`
	Model   string `yaml:"model"`
	Issue   string `yaml:"issue"`
	Score   int    `yaml:"score"`
}

// validOKFTypes is the OKF v0.1 valid type set.
var validOKFTypes = map[string]bool{
	"session":    true,
	"review":     true,
	"blog":       true,
	"blog-draft": true,
	"log":        true,
	"digest":     true,
	"reference":  true,
	"research":   true,
	"transcript": true,
	"queue":      true,
}

// RunWikiCheckOKF validates OKF v0.1 frontmatter compliance on all wiki .md files.
// It reports two kinds of issues:
//   - stray .md files at the type level (depth-2, should not exist)
//   - OKF frontmatter violations at the topic level (depth-3)
func RunWikiCheckOKF(wikiRoot string) ([]checkutil.Issue, error) {
	var issues []checkutil.Issue
	err := filepath.WalkDir(wikiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, checkutil.Issue{
				File:     slashRel(wikiRoot, path),
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("walk error: %v", walkErr),
			})

			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel := slashRel(wikiRoot, path)
		switch strings.Count(rel, "/") {
		case 2:
			// Stray .md file at type level — structural violation.
			issues = append(issues, checkutil.Issue{
				File:     rel,
				Severity: checkutil.SeverityError,
				Message:  "stray .md file at type level: " + rel,
			})
		case 3:
			// Topic-level file — check OKF v0.1 frontmatter compliance.
			issues = append(issues, checkFile(path, rel)...)
		default:
			// Depth 0 (pipeline artifacts), 1 (category-level rogue),
			// 4+ (nested subdirs like transcript/research) — skip.
		}

		return nil
	})

	return issues, err
}

// checkFile validates OKF frontmatter in a single wiki .md file.
func checkFile(path, rel string) []checkutil.Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("read error: %v", err),
		}}
	}

	var fm wikiFrontmatter
	body, err := frontmatter.Parse(strings.NewReader(string(data)), &fm)
	if err != nil {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("parse frontmatter: %v", err),
		}}
	}
	if len(body) == len(data) {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "missing frontmatter",
		}}
	}

	var issues []checkutil.Issue
	issues = append(issues, checkRequiredFields(&fm, rel)...)
	issues = append(issues, checkDateFormat(fm.Date, rel)...)
	issues = append(issues, checkTypeValidity(fm.Type, rel)...)

	return issues
}

// checkRequiredFields checks that title, date, source, and type are all non-empty.
func checkRequiredFields(fm *wikiFrontmatter, rel string) []checkutil.Issue {
	var issues []checkutil.Issue
	if strings.TrimSpace(fm.Title) == "" {
		issues = append(issues, checkutil.Issue{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "missing required field: title",
		})
	}
	if strings.TrimSpace(fm.Date) == "" {
		issues = append(issues, checkutil.Issue{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "missing required field: date",
		})
	}
	if strings.TrimSpace(fm.Source) == "" {
		issues = append(issues, checkutil.Issue{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "missing required field: source",
		})
	}
	if strings.TrimSpace(fm.Type) == "" {
		issues = append(issues, checkutil.Issue{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "missing required field: type",
		})
	}

	return issues
}

// checkDateFormat validates that the date field matches YYYY-MM-DD format.
func checkDateFormat(date, rel string) []checkutil.Issue {
	if date == "" {
		return nil
	}
	if !checkutil.DateFullPattern.MatchString(date) {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("invalid date format: %s (expected YYYY-MM-DD)", date),
		}}
	}

	return nil
}

// checkTypeValidity validates that the type field is a valid OKF v0.1 type.
func checkTypeValidity(typeVal, rel string) []checkutil.Issue {
	if typeVal == "" {
		return nil
	}
	if !validOKFTypes[typeVal] {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  "invalid OKF type: " + typeVal,
		}}
	}

	return nil
}

// slashRel returns a relative path with forward slashes.
func slashRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}
