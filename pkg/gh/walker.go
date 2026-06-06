package gh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// Event type constants for the gh YAML walker.
const (
	evUnreadable = "unreadable"
	evEmpty      = "empty"
	evFile       = "file"
	evNotArray   = "not-array"
	evSection    = "section"
	evRepo       = "repo"
)

// WalkerEvent types for the gh YAML walker.
type WalkerEvent struct {
	Section      map[string]any
	Repo         map[string]any
	Error        string
	Relation     string
	FilenameStem string
	File         string
	Type         string
	Content      string
	Errors       []string
	DocIndex     int
	SectionIndex int
	RepoIndex    int
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
			return err
		}
	}

	return nil
}

// processYAMLFile reads a single YAML file and yields the appropriate events.
func processYAMLFile(absPath, relPath string, fn func(WalkerEvent) error) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		if err2 := fn(WalkerEvent{Type: evUnreadable, File: relPath}); err2 != nil {
			return err2
		}

		return nil
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		if err2 := fn(WalkerEvent{Type: evEmpty, File: relPath}); err2 != nil {
			return err2
		}

		return nil
	}

	lineCount := len(strings.Split(strings.TrimSuffix(content, "\n"), "\n"))
	if err2 := fn(WalkerEvent{Type: evFile, File: relPath, Content: content, LineCount: lineCount}); err2 != nil {
		return err2
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
				return err
			}

			break
		}
		if doc == nil {
			continue
		}

		if err := processYAMLDoc(doc, relPath, filenameStem, fn, docIndex); err != nil {
			return err
		}
		docIndex++
	}

	return nil
}

// processYAMLDoc processes a single decoded YAML document.
func processYAMLDoc(doc any, relPath, filenameStem string, fn func(WalkerEvent) error, docIndex int) error {
	items, ok := doc.([]any)
	if !ok {
		return fn(WalkerEvent{Type: evNotArray, File: relPath, DocIndex: docIndex})
	}

	for sectionIdx, item := range items {
		section, ok := item.(map[string]any)
		if !ok {
			continue
		}

		if err := emitSectionEvent(fn, relPath, filenameStem, sectionIdx, section); err != nil {
			return err
		}

		if err := emitRepoEvents(fn, relPath, filenameStem, sectionIdx, section); err != nil {
			return err
		}
	}

	return nil
}

// emitSectionEvent yields a section event for the given section map.
func emitSectionEvent(fn func(WalkerEvent) error, relPath, filenameStem string, sectionIdx int, section map[string]any) error {
	return fn(WalkerEvent{
		Type:         evSection,
		File:         relPath,
		FilenameStem: filenameStem,
		SectionIndex: sectionIdx,
		Section:      section,
	})
}

// emitRepoEvents yields events for repo entries and the using entry in a section.
func emitRepoEvents(fn func(WalkerEvent) error, relPath, filenameStem string, sectionIdx int, section map[string]any) error {
	// Process repo entries
	if repos, ok := section["repo"].([]any); ok {
		for repoIdx, r := range repos {
			if repo, ok := r.(map[string]any); ok {
				if err2 := fn(WalkerEvent{
					Type:         evRepo,
					File:         relPath,
					FilenameStem: filenameStem,
					SectionIndex: sectionIdx,
					RepoIndex:    repoIdx,
					Relation:     evTypeRepo,
					Repo:         repo,
					Section:      section,
				}); err2 != nil {
					return err2
				}
			}
		}
	}

	// Process using entry
	if using, ok := section["using"].(map[string]any); ok {
		if err2 := fn(WalkerEvent{
			Type:         evRepo,
			File:         relPath,
			FilenameStem: filenameStem,
			SectionIndex: sectionIdx,
			RepoIndex:    0,
			Relation:     "using",
			Repo:         using,
			Section:      section,
		}); err2 != nil {
			return err2
		}
	}

	return nil
}

func collectYAMLFilesRecursive(root string) ([]string, error) {
	files, err := fileutil.ListYAMLFilesRecursive(root)
	if err != nil {
		return nil, fmt.Errorf("gh root dir: %w", err)
	}

	return files, nil
}
