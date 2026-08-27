package domrules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	modelbooks "github.com/xbpk3t/docs-alfred/internal/gh/model/books"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// DuplicateReport contains duplicate detection results.
type DuplicateReport struct {
	URLDuplicates        []URLDupEntry        `json:"urlDuplicates"`
	NameAuthorDuplicates []NameAuthorDupEntry `json:"nameAuthorDuplicates"`
}

// URLDupEntry describes a URL found in multiple entries.
type URLDupEntry struct {
	URL     string      `json:"url"`
	Entries []ItemBrief `json:"entries"`
}

// NameAuthorDupEntry describes a name+author combination found in multiple entries.
type NameAuthorDupEntry struct {
	Key     string      `json:"key"`
	Entries []ItemBrief `json:"entries"`
}

// ItemBrief is a concise representation of a data entry.
type ItemBrief struct {
	File   string `json:"file"`
	Name   string `json:"name"`
	Author string `json:"author"`
	Tags   string `json:"tags,omitempty"`
	URL    string `json:"url,omitempty"`
	Score  int    `json:"score,omitempty"`
}

// parsedItem represents a parsed YAML entry.
type parsedItem struct {
	file   string
	name   string
	author string
	url    string
	tags   []string
	score  int
}

// RunDuplicateCheck detects duplicate entries in a data directory.
func RunDuplicateCheck(targetDir string) (*DuplicateReport, error) {
	items, err := parseDomainFiles(targetDir)
	if err != nil {
		return nil, err
	}

	urlDups, urlMatchItems := groupByURL(items)
	nameAuthorDups := groupByNameAuthor(items, urlMatchItems)

	return &DuplicateReport{
		URLDuplicates:        urlDups,
		NameAuthorDuplicates: nameAuthorDups,
	}, nil
}

func groupByURL(items []parsedItem) ([]URLDupEntry, map[string]bool) {
	byURL := lo.GroupBy(lo.Filter(items, func(item parsedItem, _ int) bool {
		return item.url != ""
	}), func(item parsedItem) string {
		return item.url
	})

	urlMatchItems := make(map[string]bool)
	var entries []URLDupEntry
	for url, list := range byURL {
		if len(list) <= 1 {
			continue
		}
		entryList := lo.Map(list, func(item parsedItem, _ int) ItemBrief {
			return brief(&item)
		})
		for _, item := range list {
			urlMatchItems[item.file+":"+item.name] = true
		}
		entries = append(entries, URLDupEntry{
			URL:     url,
			Entries: entryList,
		})
	}

	return entries, urlMatchItems
}

func groupByNameAuthor(items []parsedItem, urlMatchItems map[string]bool) []NameAuthorDupEntry {
	byNameAuthor := lo.GroupBy(items, func(item parsedItem) string {
		return item.name + " | " + item.author
	})

	var entries []NameAuthorDupEntry
	for key, list := range byNameAuthor {
		if len(list) <= 1 {
			continue
		}
		uncaught := lo.Filter(list, func(item parsedItem, _ int) bool {
			return !urlMatchItems[item.file+":"+item.name]
		})
		if len(uncaught) <= 1 {
			continue
		}
		entryList := lo.Map(uncaught, func(item parsedItem, _ int) ItemBrief {
			return brief(&item)
		})
		entries = append(entries, NameAuthorDupEntry{
			Key:     key,
			Entries: entryList,
		})
	}

	return entries
}

// RunGHDuplicateCheck detects duplicate URLs in data/gh YAML files.
// It reuses the exact loader that render/export/sync/dump use
// (ghindex.LoadConfigReposFromDir → ConfigRepos.ToRepos), so the set of repos
// considered is always the same as production indexing — there is no bespoke
// parser that can drift from the real schema.
func RunGHDuplicateCheck(targetDir string) (*DuplicateReport, error) {
	configs, err := ghindex.LoadConfigReposFromDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("load gh repos from %s: %w", targetDir, err)
	}

	return groupURLDuplicates(configs.ToRepos(), targetDir), nil
}

func ghRepoRelation(repo *ghindex.Repo) string {
	if repo.IsRelatedRepo {
		return "rel"
	}
	if repo.TopicName != "" {
		return "topic:" + repo.TopicName
	}

	return "repo"
}

// groupURLDuplicates groups gh repos by a normalized repo key and returns a
// report of duplicates. It consumes the same enriched Repos the loader flattens
// for render/export, so provenance (Type/TopicName/File) is read straight off
// the model it already has.
//
// The group key is the GitHub owner/name pair (case-insensitive) rather than the
// raw URL, so variants that point at the same repo — e.g. a trailing slash
// (https://github.com/tmc/lang/ vs .../lang) or a different URL
// spelling — are collapsed into one group. The report's URL field keeps the
// first-seen spelling so messages stay readable.
func groupURLDuplicates(repos ghindex.Repos, targetDir string) *DuplicateReport {
	// Relativize File against targetDir once per source file (many repos share
	// one file) so report locations read "AI/LLM-res.yml".
	relCache := make(map[string]string)
	rel := func(path string) string {
		if r, ok := relCache[path]; ok {
			return r
		}
		out := path
		if rel, err := filepath.Rel(targetDir, path); err == nil {
			out = rel
		}
		relCache[path] = out

		return out
	}

	byKey := lo.GroupBy(lo.Filter(repos, func(repo *ghindex.Repo, _ int) bool {
		return repo != nil && repo.URL != ""
	}), func(repo *ghindex.Repo) string {
		return ghRepoURLKey(repo.URL)
	})

	report := &DuplicateReport{}
	for _, list := range byKey {
		if len(list) <= 1 {
			continue
		}
		entries := make([]ItemBrief, len(list))
		for i, repo := range list {
			entries[i] = ItemBrief{
				File: fmt.Sprintf("%s: %s (%s)", rel(repo.File), repo.Type, ghRepoRelation(repo)),
				URL:  repo.URL,
			}
		}
		// list[0] is the first-seen spelling (lo.GroupBy preserves input order).
		report.URLDuplicates = append(report.URLDuplicates, URLDupEntry{
			URL:     list[0].URL,
			Entries: entries,
		})
	}

	return report
}

// ghRepoURLKey returns a canonical key used to detect that two gh repo URLs
// refer to the same repository. For GitHub URLs it is the lowercase
// owner/name; anything else is normalized per the shared URL dedup rules
// (trailing slash, case, fragments, tracking params, …).
func ghRepoURLKey(rawURL string) string {
	if repo, ok := urlutil.GitHubOwnerRepo(rawURL); ok {
		return strings.ToLower(repo.Owner + "/" + repo.Name)
	}

	return urlutil.NormalizeForDedup(rawURL)
}

func parseDomainFiles(targetDir string) ([]parsedItem, error) {
	files, err := fileutil.ListYAMLFiles(targetDir)
	if err != nil {
		return nil, err
	}

	var items []parsedItem

	for _, docPath := range files {
		data, err := os.ReadFile(docPath)
		if err != nil {
			continue
		}

		base := filepath.Base(docPath)
		if secItems, ok := parseSectionRows(data, base); ok {
			items = append(items, secItems...)
			continue
		}

		// Fall back to the legacy flat format (top-level name/author/url rows).
		docs, err := parser.NewParser[[]yamlItem](data).ParseMulti()
		if err != nil {
			continue
		}

		for _, doc := range docs {
			items = append(items, parseYAMLDocItems(doc, base)...)
		}
	}

	return items, nil
}

// parseSectionRows extracts (name/author/url/score) rows from the flat books
// YAML by recursing topics[].table[]. The decode target is generated from
// books.schema.json (internal/gh/model/books), so the books shape stays in
// lockstep with the schema instead of a hand-mirrored struct. Returns ok=false
// when the file has no topic rows, so the caller falls back to the flat parser.
func parseSectionRows(data []byte, fileName string) ([]parsedItem, bool) {
	docs, err := parser.NewParser[[]modelbooks.Topic](data).ParseMulti()
	if err != nil {
		return nil, false
	}

	var items []parsedItem
	// Only the topic-shaped layout (each topic holding a table of rows) counts
	// as a section; bare top-level name/author rows fall back to the legacy flat
	// parser below.
	hasSection := false
	for _, doc := range docs {
		for _, topic := range doc {
			for _, row := range topic.Table {
				hasSection = true
				if row.Name == "" {
					continue
				}
				items = append(items, parsedItem{
					file:   fileName,
					name:   row.Name,
					author: derefStr(row.Author),
					url:    derefStr(row.URL),
					score:  derefInt(row.Score),
				})
			}
		}
	}

	if !hasSection {
		return nil, false
	}

	return items, true
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}

	return *p
}

// toTags normalizes a tags value (string or []string) into a []string.
func toTags(v any) []string {
	switch t := v.(type) {
	case string:
		if t != "" {
			return []string{t}
		}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}

	return nil
}

type yamlItem struct {
	Tags   any    `yaml:"tags"`
	Name   string `yaml:"name"`
	Author string `yaml:"author"`
	URL    string `yaml:"url"`
	Score  int    `yaml:"score"`
}

func parseYAMLDocItems(doc []yamlItem, fileName string) []parsedItem {
	var items []parsedItem
	for _, item := range doc {
		if item.Name == "" {
			continue
		}
		items = append(items, parsedItem{
			file:   fileName,
			name:   item.Name,
			author: item.Author,
			score:  item.Score,
			tags:   toTags(item.Tags),
			url:    item.URL,
		})
	}

	return items
}

func brief(item *parsedItem) ItemBrief {
	return ItemBrief{
		File:   item.file,
		Name:   item.name,
		Author: item.author,
		URL:    item.url,
	}
}

// FormatDuplicateReport returns a human-readable string of the report.
func FormatDuplicateReport(report *DuplicateReport) string {
	return formatDuplicateReport("data duplicate", report.issues(false))
}

// FormatGHDuplicateReport returns a human-readable string of the gh duplicate report.
func FormatGHDuplicateReport(report *DuplicateReport) string {
	return formatDuplicateReport("data gh duplicate", report.issues(true))
}

func formatDuplicateReport(name string, issues []checkutil.Issue) string {
	result := &checkutil.Result{Issues: issues}

	return result.ReportResult(name)
}

func (r *DuplicateReport) issues(ghOnly bool) []checkutil.Issue {
	if r == nil {
		return nil
	}

	var issues []checkutil.Issue
	for _, dup := range r.URLDuplicates {
		issues = append(issues, checkutil.Issue{
			File:     "duplicate",
			Severity: checkutil.SeverityError,
			Message:  "重复 URL: " + dup.URL + formatDuplicateEntries(dup.Entries, ghOnly),
		})
	}

	if !ghOnly {
		for _, dup := range r.NameAuthorDuplicates {
			issues = append(issues, checkutil.Issue{
				File:     "duplicate",
				Severity: checkutil.SeverityError,
				Message:  "重复名称+作者: " + dup.Key + formatDuplicateEntries(dup.Entries, false),
			})
		}
	}

	return issues
}

func formatDuplicateEntries(entries []ItemBrief, ghOnly bool) string {
	var lines []string
	for _, entry := range entries {
		if ghOnly {
			lines = append(lines, "  -> "+entry.File)

			continue
		}

		lines = append(lines, fmt.Sprintf("  -> %s: %s - %s", entry.File, entry.Name, entry.Author))
	}
	if len(lines) == 0 {
		return ""
	}

	return "\n" + strings.Join(lines, "\n")
}
