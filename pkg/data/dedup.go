package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
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

//nolint:gocritic // groupByURL returns unnamed results; named returns conflict with nonamedreturns
func groupByURL(items []parsedItem) ([]URLDupEntry, map[string]bool) {
	byURL := make(map[string][]parsedItem)
	for _, item := range items {
		if item.url == "" {
			continue
		}
		byURL[item.url] = append(byURL[item.url], item)
	}

	urlMatchItems := make(map[string]bool)
	var entries []URLDupEntry
	for url, list := range byURL {
		if len(list) <= 1 {
			continue
		}
		entryList := make([]ItemBrief, len(list))
		for i, item := range list {
			entryList[i] = brief(&item)
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
	byNameAuthor := make(map[string][]parsedItem)
	for _, item := range items {
		key := item.name + " | " + item.author
		byNameAuthor[key] = append(byNameAuthor[key], item)
	}

	var entries []NameAuthorDupEntry
	for key, list := range byNameAuthor {
		if len(list) <= 1 {
			continue
		}
		var uncaught []parsedItem
		for _, item := range list {
			if !urlMatchItems[item.file+":"+item.name] {
				uncaught = append(uncaught, item)
			}
		}
		if len(uncaught) <= 1 {
			continue
		}
		entryList := make([]ItemBrief, len(uncaught))
		for i, item := range uncaught {
			entryList[i] = brief(&item)
		}
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

// collectGhRepoEntries reads all gh YAML files and collects repo/using entries with URLs.
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
		yamlFiles, err := filepath.Glob(filepath.Join(dirPath, "*.yml"))
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

// parseGhYAMLEntries extracts repo/using URL entries from a single gh YAML file.
func parseGhYAMLEntries(yf, targetDir string) ([]ghEntry, error) {
	data, err := os.ReadFile(yf)
	if err != nil {
		return nil, err
	}

	var items []map[string]any
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	relFile, _ := filepath.Rel(targetDir, yf)

	var entries []ghEntry
	for _, item := range items {
		typeName, _ := item["type"].(string)
		if typeName == "" {
			continue
		}

		entries = appendGhURLEntries(entries, relFile, typeName, item)
	}

	return entries, nil
}

// appendGhURLEntries extracts 'using' and 'repo' URLs from a parsed YAML item.
func appendGhURLEntries(entries []ghEntry, relFile, typeName string, item map[string]any) []ghEntry {
	// Check 'using'
	if using, ok := item["using"].(map[string]any); ok {
		if u, ok := using["url"].(string); ok && u != "" {
			entries = append(entries, ghEntry{
				file: relFile, typeName: typeName, relation: "using", url: u,
			})
		}
	}

	// Check 'repo' array
	if repos, ok := item["repo"].([]any); ok {
		for _, r := range repos {
			if repo, ok := r.(map[string]any); ok {
				if u, ok := repo["url"].(string); ok && u != "" {
					entries = append(entries, ghEntry{
						file: relFile, typeName: typeName, relation: "repo", url: u,
					})
				}
			}
		}
	}

	return entries
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
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", targetDir, err)
	}

	var items []parsedItem

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != extYML && ext != extYAML {
			continue
		}

		docPath := filepath.Join(targetDir, e.Name())
		data, err := os.ReadFile(docPath)
		if err != nil {
			continue
		}

		// Parse multi-document YAML using the generic parser
		docs, err := parser.NewParser[[]map[string]any](data).ParseMulti()
		if err != nil {
			continue
		}

		for _, doc := range docs {
			items = append(items, parseYAMLDocItems(doc, e.Name())...)
		}
	}

	return items, nil
}

func parseYAMLDocItems(doc []map[string]any, fileName string) []parsedItem {
	var items []parsedItem
	for _, item := range doc {
		name, _ := item["name"].(string)
		if name == "" {
			continue
		}
		author, _ := item["author"].(string)
		url, _ := item["url"].(string)
		score := 0
		if s, ok := item["score"].(int); ok {
			score = s
		}

		var tags []string
		if tagList, ok := item["tags"].([]any); ok {
			for _, t := range tagList {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		}

		items = append(items, parsedItem{
			file:   fileName,
			name:   name,
			author: author,
			score:  score,
			tags:   tags,
			url:    url,
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
	var lines []string

	for _, dup := range report.URLDuplicates {
		lines = append(lines, "ERROR 重复 URL: "+dup.URL)
		for _, entry := range dup.Entries {
			lines = append(lines, fmt.Sprintf("  → %s: %s — %s", entry.File, entry.Name, entry.Author))
		}
	}

	for _, dup := range report.NameAuthorDuplicates {
		lines = append(lines, "ERROR 重复名称+作者: "+dup.Key)
		for _, entry := range dup.Entries {
			lines = append(lines, fmt.Sprintf("  → %s: %s — %s", entry.File, entry.Name, entry.Author))
		}
	}

	return strings.Join(lines, "\n")
}

// FormatGHDuplicateReport returns a human-readable string of the gh duplicate report.
func FormatGHDuplicateReport(report *DuplicateReport) string {
	var lines []string
	for _, dup := range report.URLDuplicates {
		lines = append(lines, "ERROR 重复 URL: "+dup.URL)
		for _, entry := range dup.Entries {
			lines = append(lines, "  → "+entry.File)
		}
	}

	return strings.Join(lines, "\n")
}
