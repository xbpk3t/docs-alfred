package blog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// CheckResult holds the blog check result.
type CheckResult struct {
	Issues   []checkutil.Issue
	GHTypes  int
	BlogDirs int
}

// CheckIssue represents a single check issue.
type CheckIssue = checkutil.Issue

// RunCheck compares blog directories with data/gh YAML files.
func RunCheck(dataDir, blogDir string) (*CheckResult, error) {
	result := &CheckResult{}

	ghTypes, err := collectGHTypes(result, dataDir)
	if err != nil {
		return nil, err
	}

	blogDirs, err := collectBlogDirs(result, blogDir)
	if err != nil {
		return nil, err
	}

	// Check for blog dirs without gh types
	for key := range blogDirs {
		if !ghTypes[key] {
			result.Issues = append(result.Issues, checkutil.Issue{
				File: key, Severity: "error",
				Message: "blog directory exists but no corresponding data/gh YAML file",
			})
		}
	}

	return result, nil
}

func collectGHTypes(result *CheckResult, dataDir string) (map[string]bool, error) {
	ghTypes := make(map[string]bool)
	err := filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		rel, _ := filepath.Rel(dataDir, path)
		dirName := filepath.Dir(rel)
		stem := strings.TrimSuffix(filepath.Base(path), ext)
		key := dirName + "/" + stem
		if strings.HasPrefix(dirName, ".") {
			return nil
		}
		ghTypes[key] = true
		result.GHTypes++

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk data/gh: %w", err)
	}

	return ghTypes, nil
}

func collectBlogDirs(result *CheckResult, blogDir string) (map[string]bool, error) {
	skipDirs := map[string]bool{"static": true}
	blogDirs := make(map[string]bool)

	entries, err := os.ReadDir(blogDir)
	if err != nil {
		// blog dir may not exist; surface other errors — that's fine
		if !os.IsNotExist(err) {
			return nil, err
		}

		return blogDirs, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || skipDirs[entry.Name()] {
			continue
		}
		folderPath := filepath.Join(blogDir, entry.Name())
		subEntries, err := os.ReadDir(folderPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			key := entry.Name() + "/" + sub.Name()
			blogDirs[key] = true
			result.BlogDirs++
		}
	}

	return blogDirs, nil
}

// Report prints the check result.
func (r *CheckResult) Report(command string) {
	base := &checkutil.Result{Issues: r.Issues}
	base.Report(command)
	fmt.Fprintf(os.Stderr, "summary: data/gh types=%d blog dirs=%d\n", r.GHTypes, r.BlogDirs)
}

// HasErrors returns true if there are any error-severity issues.
func (r *CheckResult) HasErrors() bool {
	return checkutil.HasErrors(r.Issues)
}
