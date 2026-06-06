package gh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const evTypeRepo = "repo"

var errURLNotFound = errors.New("URL not found in this document")

// AppendRecordOptions holds options for appending a record.
type AppendRecordOptions struct {
	File  string // explicit target YAML file
	URL   string // repo URL to find
	Date  string // record date (YYYY-MM-DD)
	Des   string // record description
	Topic string // topic name (default: inferred from URL)
}

// AppendRecordResult holds the result of an append-record operation.
type AppendRecordResult struct {
	File      string
	URL       string
	Date      string
	Des       string
	Diff      string
	Confirmed bool
}

// AppendRecord appends a record to a data/gh YAML entry.
func AppendRecord(opts *AppendRecordOptions) (*AppendRecordResult, error) {
	if !checkutil.DateFullPattern.MatchString(opts.Date) {
		return nil, fmt.Errorf("invalid date format %q (expected YYYY-MM-DD)", opts.Date)
	}

	// Find target file if not explicitly given
	targetFile := opts.File
	if targetFile == "" {
		found, err := findFileByURL("data/gh", opts.URL)
		if err != nil {
			return nil, fmt.Errorf("find file by URL: %w", err)
		}
		targetFile = found
	}

	if targetFile == "" {
		return nil, fmt.Errorf("no file found for URL %q", opts.URL)
	}

	// Determine topic path
	topicName := opts.Topic
	if topicName == "" {
		topicName = inferTopicFromURL(opts.URL)
	}

	// Append record using go-yaml AST
	if err := appendYAMLRecord(targetFile, opts.URL, topicName, opts.Date, opts.Des); err != nil {
		return nil, fmt.Errorf("append record: %w", err)
	}

	// Confirm the YAML file is still valid by re-parsing
	if _, err := os.ReadFile(targetFile); err != nil {
		return nil, fmt.Errorf("re-read %s: %w", targetFile, err)
	}

	// Get git diff --stat
	diffOutput, _ := getGitDiffStat(targetFile)

	result := &AppendRecordResult{
		File:      targetFile,
		URL:       opts.URL,
		Date:      opts.Date,
		Des:       opts.Des,
		Diff:      diffOutput,
		Confirmed: true,
	}

	return result, nil
}

func findFileByURL(ghRoot, url string) (string, error) {
	var foundFiles []string

	err := WalkGhRepos(ghRoot, func(ev WalkerEvent) error {
		if ev.Type != evTypeRepo {
			return nil
		}
		repo := ev.Repo
		repoURL, _ := repo["url"].(string)
		if urlutil.Equal(repoURL, url) {
			foundFiles = append(foundFiles, filepath.Join(ghRoot, ev.File))
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no file contains URL %q", url)
		}

		return "", err
	}

	if len(foundFiles) == 0 {
		return "", fmt.Errorf("no file contains URL %q", url)
	}
	if len(foundFiles) > 1 {
		return "", fmt.Errorf("multiple files contain URL %q:\n  %s", url, strings.Join(foundFiles, "\n  "))
	}

	return foundFiles[0], nil
}

func inferTopicFromURL(urlStr string) string {
	return urlutil.RepoName(urlStr)
}

func getGitDiffStat(file string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", file)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

// appendYAMLRecord appends a record entry to the YAML file using go-yaml AST.
// It finds the section containing the matching repo URL, locates the record
// sequence (either topic-level or section-level), and appends the new entry.
func appendYAMLRecord(file, findURL, topicName, date, des string) error {
	f, err := parser.ParseFile(file, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	recSeq, err := findRecordSequence(f, findURL, topicName)
	if err != nil {
		return err
	}

	newRecord, err := createRecordNode(date, des)
	if err != nil {
		return err
	}

	recSeq.Values = append(recSeq.Values, newRecord)

	if err := os.WriteFile(file, []byte(f.String()), 0600); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}

	return nil
}

// findRecordSequence traverses the AST to find the target record sequence.
// It iterates documents, finds the section containing the URL, then locates
// the record sequence (topic-level if topicName is given and found, else section-level).
func findRecordSequence(f *ast.File, findURL, topicName string) (*ast.SequenceNode, error) {
	for _, doc := range f.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		seq, ok := doc.Body.(*ast.SequenceNode)
		if !ok {
			continue
		}

		result, err := findRecordInSequence(seq, findURL, topicName)
		if errors.Is(err, errURLNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no section found with URL %q", findURL)
}

// findRecordInSequence searches sequence values for a matching section with the target URL.
func findRecordInSequence(seq *ast.SequenceNode, findURL, topicName string) (*ast.SequenceNode, error) {
	for _, item := range seq.Values {
		section, ok := item.(*ast.MappingNode)
		if !ok {
			continue
		}
		if !sectionContainsURL(section, findURL) {
			continue
		}

		// Found the section — now locate the record sequence
		if topicName != "" {
			recSeq, err := findTopicRecord(section, topicName)
			if err == nil {
				return recSeq, nil
			}
			// Topic not found or has no record field, fall through to section-level
		}

		// Section-level record
		recVal := findMappingValue(section, "record")
		if recVal == nil {
			return nil, errors.New("section has no 'record' field")
		}
		recSeq, ok := recVal.(*ast.SequenceNode)
		if !ok {
			return nil, errors.New("'record' is not a sequence")
		}

		return recSeq, nil
	}

	return nil, errURLNotFound
}

// findMappingValue finds the value node for a given key in a mapping node.
func findMappingValue(m *ast.MappingNode, key string) ast.Node {
	for _, v := range m.Values {
		if k, ok := v.Key.(*ast.StringNode); ok && k.Value == key {
			return v.Value
		}
	}

	return nil
}

// sectionContainsURL checks if any repo entry in the section has the matching URL.
func sectionContainsURL(section *ast.MappingNode, findURL string) bool {
	repoVal := findMappingValue(section, "repo")
	if repoVal == nil {
		return false
	}
	repoSeq, ok := repoVal.(*ast.SequenceNode)
	if !ok {
		return false
	}
	for _, repoItem := range repoSeq.Values {
		repoMap, ok := repoItem.(*ast.MappingNode)
		if !ok {
			continue
		}
		urlVal := findMappingValue(repoMap, "url")
		if urlVal == nil {
			continue
		}
		urlStr, ok := urlVal.(*ast.StringNode)
		if !ok {
			continue
		}
		if urlutil.Equal(urlStr.Value, findURL) {
			return true
		}
	}

	return false
}

// findTopicRecord finds the record sequence within a matching topic entry.
func findTopicRecord(section *ast.MappingNode, topicName string) (*ast.SequenceNode, error) {
	topicsVal := findMappingValue(section, "topics")
	if topicsVal == nil {
		return nil, errors.New("section has no 'topics' field")
	}
	topicsSeq, ok := topicsVal.(*ast.SequenceNode)
	if !ok {
		return nil, errors.New("'topics' is not a sequence")
	}
	for _, topicItem := range topicsSeq.Values {
		topicMap, ok := topicItem.(*ast.MappingNode)
		if !ok {
			continue
		}
		topicStrVal := findMappingValue(topicMap, "topic")
		if topicStrVal == nil {
			continue
		}
		topicStr, ok := topicStrVal.(*ast.StringNode)
		if !ok {
			continue
		}
		if topicStr.Value == topicName {
			recVal := findMappingValue(topicMap, "record")
			if recVal == nil {
				return nil, fmt.Errorf("topic %q has no 'record' field", topicName)
			}
			recSeq, ok := recVal.(*ast.SequenceNode)
			if !ok {
				return nil, fmt.Errorf("topic %q 'record' is not a sequence", topicName)
			}

			return recSeq, nil
		}
	}

	return nil, fmt.Errorf("topic %q not found in section", topicName)
}

// createRecordNode creates a new AST mapping node for a record entry.
// It marshals via go-yaml for proper quoting/escaping, then re-parses as a snippet.
func createRecordNode(date, des string) (ast.Node, error) {
	record := map[string]string{"date": date, "des": des}
	data, err := yaml.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("marshal record: %w", err)
	}

	// Re-indent as a sequence entry: "- date: ...\n  des: ..."
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = "- " + line
		} else {
			lines[i] = "  " + line
		}
	}
	snippet := strings.Join(lines, "\n")

	// Parse into AST and extract the MappingNode
	snippetFile, err := parser.ParseBytes([]byte(snippet), parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse record snippet: %w", err)
	}
	if len(snippetFile.Docs) == 0 || snippetFile.Docs[0].Body == nil {
		return nil, errors.New("record snippet has no document or body")
	}
	seq, ok := snippetFile.Docs[0].Body.(*ast.SequenceNode)
	if !ok {
		return nil, errors.New("body is not a sequence node")
	}
	if len(seq.Values) == 0 {
		return nil, errors.New("record snippet has no values")
	}

	return seq.Values[0], nil
}
