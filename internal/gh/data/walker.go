package ghdata

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// Event type constants for the gh YAML walker.
const (
	evUnreadable = "unreadable"
	evEmpty      = "empty"
	evFile       = "file"
	evNotArray   = "not-array"
	evSection    = "section"
)

// WalkerEvent types for the gh YAML walker.
type WalkerEvent struct {
	Section      Section
	Error        string
	FilenameStem string
	File         string
	Content      string
	Type         string
	DocIndex     int
	SectionIndex int
	LineCount    int
}

// WalkGhRepos walks all YAML files in ghRoot and yields events.
func WalkGhRepos(ghRoot string, fn func(WalkerEvent) error) error {
	yamlFiles, err := collectYAMLFilesRecursive(ghRoot)
	if err != nil {
		return fmt.Errorf("walk gh repos: %w", err)
	}

	for _, absPath := range yamlFiles {
		relPath, _ := filepath.Rel(ghRoot, absPath)

		if err := processYAMLFile(absPath, relPath, fn); err != nil {
			return fmt.Errorf("process %s: %w", relPath, err)
		}
	}

	return nil
}

// processYAMLFile reads a single YAML file and yields the appropriate events.
func processYAMLFile(absPath, relPath string, fn func(WalkerEvent) error) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		if err2 := fn(WalkerEvent{Type: evUnreadable, File: relPath}); err2 != nil {
			return fmt.Errorf("callback for unreadable %s: %w", relPath, err2)
		}

		return nil
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		if err2 := fn(WalkerEvent{Type: evEmpty, File: relPath}); err2 != nil {
			return fmt.Errorf("callback for empty %s: %w", relPath, err2)
		}

		return nil
	}

	lineCount := len(strings.Split(strings.TrimSuffix(content, "\n"), "\n"))
	if err2 := fn(WalkerEvent{Type: evFile, File: relPath, Content: content, LineCount: lineCount}); err2 != nil {
		return fmt.Errorf("callback for file %s: %w", relPath, err2)
	}

	filenameStem := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))

	return processYAMLContent(content, relPath, filenameStem, fn)
}

func processYAMLContent(content, relPath, filenameStem string, fn func(WalkerEvent) error) error {
	decoder := yaml.NewDecoder(strings.NewReader(content))
	docIndex := 0

	for {
		var doc any
		if err := decoder.Decode(&doc); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("decode YAML doc %d in %s: %w", docIndex, relPath, err)
			}

			break
		}
		if doc == nil {
			continue
		}

		if err := processYAMLDoc(doc, relPath, filenameStem, fn, docIndex); err != nil {
			return fmt.Errorf("process doc %d in %s: %w", docIndex, relPath, err)
		}
		docIndex++
	}

	return nil
}

// processYAMLDoc processes a single decoded YAML document.
//
// In the new data/gh layout a file's root is a flat topic array and the whole
// file is one section (type derived from the file name). Each document therefore
// becomes a single section event whose topics are the document's items. The
// empty-topic (non-mapping) entries are skipped.
func processYAMLDoc(doc any, relPath, filenameStem string, fn func(WalkerEvent) error, docIndex int) error {
	items, ok := doc.([]any)
	if !ok {
		return fn(WalkerEvent{Type: evNotArray, File: relPath, DocIndex: docIndex})
	}

	var topics []Topic
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		topics = append(topics, topicFromMap(m))
	}
	if len(topics) == 0 {
		return nil
	}

	return emitSectionEvent(fn, relPath, filenameStem, topics)
}

// emitSectionEvent yields a section event. The section type is derived from the
// file name stem (algo.yml → "algo"); its topics are the whole topic array. In
// the flat layout a file is always a single section, so SectionIndex is 0.
func emitSectionEvent(fn func(WalkerEvent) error, relPath, filenameStem string, topics []Topic) error {
	derived := gh.TypeFromFilename(filenameStem)

	return fn(WalkerEvent{
		Type:         evSection,
		File:         relPath,
		FilenameStem: filenameStem,
		SectionIndex: 0,
		Section: Section{
			Type:   &derived,
			Topics: topics,
		},
	})
}

func collectYAMLFilesRecursive(root string) ([]string, error) {
	files, err := fileutil.ListYAMLFilesRecursive(root)
	if err != nil {
		return nil, fmt.Errorf("gh root dir: %w", err)
	}

	return files, nil
}
