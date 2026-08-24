package workspaceops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// wikiFrontmatter represents the OKF v0.1 frontmatter fields expected in wiki .md files.
// Source, Session, Model, Issue, and Score are optional fields written by the CLI
// pipelines (ccx session export etc.) or edited manually; they are parsed for
// forward-compat but are never required.
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
// Single source of truth lives in internal/docs/wiki/types (ClassifyType).
// Pipeline-only types (review/inbox) are NOT valid OKF types.
var validOKFTypes = map[string]bool{
	string(wikitypes.TypeBlog):     true,
	string(wikitypes.TypeLog):      true,
	string(wikitypes.TypeDigest):   true,
	string(wikitypes.TypeDeepDive): true, // "research"
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
		// Transcript productions (…/transcript/*.md) are pipeline artifacts,
		// not OKF wiki content: skip them entirely, check or stats.
		if checkutil.HasSegmentDir(rel, wikitypes.ArtifactDir) {
			return nil
		}
		depth := strings.Count(rel, "/")
		base := d.Name()

		// Fixed-name per-topic artifacts (summary.md / log.md) are only legal
		// at exactly <tag>/<type>/<topic>/ (their declared depth). Anywhere
		// else is a structural violation, not validated as content.
		if art, ok := topicArtifacts[base]; ok && depth != art.depth {
			issues = append(issues, checkutil.Issue{
				File:     rel,
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("%s only allowed at <tag>/<type>/<topic>/: %s", base, rel),
			})
			return nil
		}

		switch {
		case depth == 2:
			// Stray .md file at type level — structural violation.
			issues = append(issues, checkutil.Issue{
				File:     rel,
				Severity: checkutil.SeverityError,
				Message:  "stray .md file at type level: " + rel,
			})
		case depth == 3:
			// Topic-level file — check OKF v0.1 frontmatter compliance.
			issues = append(issues, checkFile(path, rel)...)
		case depth > 3 && isBlogEntry(rel):
			// A file under a blog/ dir is a blog entry, wherever it nests.
			issues = append(issues, checkFile(path, rel)...)
		default:
			// Depth 0/1 (pipeline artifacts, category-level rogue) and 4+
			// non-blog nested subdirs (transcript/research/qa etc.) — skip.
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
	issues = append(issues, checkTypeConsistency(&fm, rel)...)

	return issues
}

// checkRequiredFields checks that title, date, and type are all non-empty.
// Source is optional (forward-compat), like session/model: entries such as
// hand-written notes, digests, or inbox seeds may not have a meaningful source.
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

// topicArtifact is a fixed-name per-topic file: the only depth it may live at
// and the OKF type its frontmatter must carry. Both the walk's placement rule
// and the content binding derive from this single table, so a new per-topic
// artifact can't drift between the two checks.
type topicArtifact struct {
	typ   wikitypes.ClassifyType
	depth int
}

var topicArtifacts = map[string]topicArtifact{
	"summary.md": {depth: 3, typ: wikitypes.TypeDigest},
	"log.md":     {depth: 3, typ: wikitypes.TypeLog},
}

// isBlogEntry reports whether rel roots under a <topic>/blog/ directory.
func isBlogEntry(rel string) bool { return checkutil.HasSegmentDir(rel, wikitypes.BlogDir) }

// expectedTypeFor returns the frontmatter type the file's name/location
// requires, or "" when there is no binding:
//
//	summary→digest, log→log, files under a blog/ dir→blog
func expectedTypeFor(rel string) string {
	if art, ok := topicArtifacts[filepath.Base(rel)]; ok {
		return string(art.typ)
	}
	if isBlogEntry(rel) {
		return string(wikitypes.TypeBlog)
	}
	return ""
}

// checkTypeConsistency enforces the name/location → type binding: summary.md
// must be type=digest, log.md must be type=log, and every file under blog/
// must be type=blog.
func checkTypeConsistency(fm *wikiFrontmatter, rel string) []checkutil.Issue {
	expected := expectedTypeFor(rel)
	if expected == "" || strings.TrimSpace(fm.Type) == "" {
		return nil
	}
	if fm.Type != expected {
		return []checkutil.Issue{{
			File:     rel,
			Severity: checkutil.SeverityError,
			Message:  fmt.Sprintf("type does not match file: %s must have type=%s (got %s)", rel, expected, fm.Type),
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
