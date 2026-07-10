package domrules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
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

// ghEntry represents a single repo entry extracted from a gh YAML file.
type ghEntry struct {
	file     string
	typeName string
	relation string
	url      string
}

// RunGHDuplicateCheck detects duplicate URLs in data/gh YAML files.
func RunGHDuplicateCheck(targetDir string) (*DuplicateReport, error) {
	repoEntries, err := collectGhRepoEntries(targetDir)
	if err != nil {
		return nil, err
	}

	return groupURLDuplicates(repoEntries), nil
}

// collectGhRepoEntries reads all gh YAML files and collects repo entries with URLs.
func collectGhRepoEntries(targetDir string) ([]ghEntry, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", targetDir, err)
	}

	var repoEntries []ghEntry

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirPath := filepath.Join(targetDir, entry.Name())
		yamlFiles, err := fileutil.ListYAMLFiles(dirPath)
		if err != nil {
			continue
		}

		for _, yf := range yamlFiles {
			fileEntries, err := parseGhYAMLEntries(yf, targetDir)
			if err != nil {
				continue
			}

			repoEntries = append(repoEntries, fileEntries...)
		}
	}

	return repoEntries, nil
}

// parseGhYAMLEntries extracts repo URL entries from a single gh YAML file.
func parseGhYAMLEntries(yf, targetDir string) ([]ghEntry, error) {
	data, err := os.ReadFile(yf)
	if err != nil {
		return nil, err
	}

	var items []ghYAMLItem
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	relFile, _ := filepath.Rel(targetDir, yf)

	var entries []ghEntry
	for _, item := range items {
		if item.Type == "" {
			continue
		}
		for _, repo := range item.Repos {
			if repo.URL != "" {
				entries = append(entries, ghEntry{file: relFile, typeName: item.Type, relation: "repo", url: repo.URL})
			}
		}
	}

	return entries, nil
}

type ghYAMLItem struct {
	Type  string       `yaml:"type"`
	Repos []ghYAMLRepo `yaml:"repo"`
}

type ghYAMLRepo struct {
	URL string `yaml:"url"`
}

// groupURLDuplicates groups gh entries by URL and returns a report of duplicates.
func groupURLDuplicates(repoEntries []ghEntry) *DuplicateReport {
	byURL := make(map[string][]ghEntry)
	for _, e := range repoEntries {
		byURL[e.url] = append(byURL[e.url], e)
	}

	report := &DuplicateReport{}
	for url, list := range byURL {
		if len(list) <= 1 {
			continue
		}
		entries := make([]ItemBrief, len(list))
		for i, e := range list {
			entries[i] = ItemBrief{
				File: fmt.Sprintf("%s: %s (%s)", e.file, e.typeName, e.relation),
				URL:  e.url,
			}
		}
		report.URLDuplicates = append(report.URLDuplicates, URLDupEntry{
			URL:     url,
			Entries: entries,
		})
	}

	return report
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

		docs, err := parser.NewParser[[]yamlItem](data).ParseMulti()
		if err != nil {
			continue
		}

		for _, doc := range docs {
			items = append(items, parseYAMLDocItems(doc, filepath.Base(docPath))...)
		}
	}

	return items, nil
}

type yamlItem struct {
	Name   string   `yaml:"name"`
	Author string   `yaml:"author"`
	URL    string   `yaml:"url"`
	Tags   []string `yaml:"tags"`
	Score  int      `yaml:"score"`
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
			tags:   item.Tags,
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
