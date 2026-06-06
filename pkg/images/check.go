package images

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	ghlib "github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// Patterns for duplicate file detection: name__NUMBER.ext.
var duplicateFileRe = regexp.MustCompile(`^(.+[^_])__\d+(\.[^.]+)$`)

// CheckConfig holds the images check configuration.
type CheckConfig struct {
	DataDir     string
	ImagesDir   string
	Apply       bool
	List        bool
	SkipExtra   bool
	SkipMissing bool
}

// CheckResult holds the images check result.
type CheckResult struct {
	ExpectedDirs   []string
	ExistingDirs   []string
	MissingDirs    []string
	ExtraDirs      []string
	DuplicateFiles []string
	Warnings       []string
	Errors         []string
	ActualFiles    []string
	ApplyActions   []string
}

// RunImagesCheck checks docs-images against data/gh expectations.
func RunImagesCheck(cfg CheckConfig) (*CheckResult, error) {
	result := &CheckResult{}

	expectedDirs, err := collectExpectedImageDirs(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("collect expected dirs: %w", err)
	}
	result.ExpectedDirs = expectedDirs

	existingDirs, actualFiles, err := collectExistingFilesAndDirs(cfg.ImagesDir)
	if err != nil {
		return nil, fmt.Errorf("collect existing dirs: %w", err)
	}
	result.ExistingDirs = existingDirs
	result.ActualFiles = actualFiles

	expectedSet := make(map[string]bool)
	for _, d := range expectedDirs {
		expectedSet[d] = true
	}
	existingSet := make(map[string]bool)
	for _, d := range existingDirs {
		existingSet[d] = true
	}

	// Find missing
	for _, d := range expectedDirs {
		if !existingSet[d] {
			result.MissingDirs = append(result.MissingDirs, d)
		}
	}

	// Find extra
	for _, d := range existingDirs {
		if !expectedSet[d] {
			result.ExtraDirs = append(result.ExtraDirs, d)
		}
	}

	// Apply fixes
	if cfg.Apply {
		applyFixes(result, cfg)
	}

	return result, nil
}

func applyFixes(result *CheckResult, cfg CheckConfig) {
	removed := removeDuplicateFiles(cfg.ImagesDir, result.ActualFiles)
	if removed > 0 {
		msg := fmt.Sprintf("Removed %d duplicate file(s)", removed)
		result.ApplyActions = append(result.ApplyActions, msg)
	}

	hidden := hideExtraDirs(cfg.ImagesDir, result.ExtraDirs)
	if hidden > 0 {
		msg := fmt.Sprintf("Hidden %d extra director(ies)", hidden)
		result.ApplyActions = append(result.ApplyActions, msg)
	}

	moved := moveExtraFiles(cfg.ImagesDir, result.ExtraDirs, result.ActualFiles)
	if moved > 0 {
		msg := fmt.Sprintf("Moved %d extra file(s) to .temp", moved)
		result.ApplyActions = append(result.ApplyActions, msg)
	}

	if len(result.ApplyActions) == 0 {
		result.ApplyActions = append(result.ApplyActions, "No fixes needed")
	}
}

// removeDuplicateFiles deletes files matching name__NUMBER.ext if name.ext exists.
func removeDuplicateFiles(imagesDir string, actualFiles []string) int {
	removed := 0
	for _, f := range actualFiles {
		base := filepath.Base(f)
		dir := filepath.Dir(f)
		matches := duplicateFileRe.FindStringSubmatch(base)
		if matches == nil {
			continue
		}
		originalName := matches[1] + matches[2]
		originalPath := filepath.Join(imagesDir, dir, originalName)
		if _, err := os.Stat(originalPath); err == nil {
			dupPath := filepath.Join(imagesDir, f)
			if err := os.Remove(dupPath); err == nil {
				removed++
			}
		}
	}

	return removed
}

// hideExtraDirs renames extra directories by prefixing with ".".
func hideExtraDirs(imagesDir string, extraDirs []string) int {
	hidden := 0
	for _, d := range extraDirs {
		oldPath := filepath.Join(imagesDir, d)

		hiddenName := "." + filepath.Base(d)
		parentDir := filepath.Dir(d)
		newPath := filepath.Join(imagesDir, parentDir, hiddenName)

		// If hidden name exists, try .1, .2, etc.
		if _, err := os.Stat(newPath); err == nil {
			for i := 1; ; i++ {
				altName := fmt.Sprintf(".%s.%d", filepath.Base(d), i)
				altPath := filepath.Join(imagesDir, parentDir, altName)
				if _, err := os.Stat(altPath); os.IsNotExist(err) {
					newPath = altPath

					break
				}
			}
		}

		if err := os.Rename(oldPath, newPath); err == nil {
			hidden++
		}
	}

	return hidden
}

// moveExtraFiles moves files in extra directories or root level to parent .temp/.
func moveExtraFiles(imagesDir string, extraDirs, actualFiles []string) int {
	extraDirSet := make(map[string]bool)
	for _, d := range extraDirs {
		extraDirSet[d] = true
	}

	moved := 0
	tempDir := filepath.Join(filepath.Dir(imagesDir), ".temp")
	_ = os.MkdirAll(tempDir, 0750)

	for _, f := range actualFiles {
		fileDir := filepath.Dir(f)
		// Skip files in expected dirs; only move files in extra dirs or root
		if fileDir != "." && !extraDirSet[fileDir] {
			continue
		}
		if strings.HasPrefix(filepath.Base(f), ".") {
			continue
		}

		src := filepath.Join(imagesDir, f)
		dst := filepath.Join(tempDir, filepath.Base(f))
		if err := os.Rename(src, dst); err == nil {
			moved++
		}
	}

	return moved
}

func collectExpectedImageDirs(dataDir string) ([]string, error) {
	var dirs []string

	err := ghlib.WalkGhRepos(dataDir, func(ev ghlib.WalkerEvent) error {
		if ev.Type != "section" {
			return nil
		}

		section := ev.Section
		typeVal, _ := section["type"].(string)
		if typeVal == "" {
			return nil
		}

		// Infer tag from directory structure
		dirParts := strings.Split(ev.File, string(filepath.Separator))
		if len(dirParts) < 2 {
			return nil
		}
		tag := dirParts[0]
		typeBase := tag + "/" + typeVal

		// Collect topic dirs
		if topics, ok := section["topics"].([]any); ok {
			collectTopicDirs(ev.File, topics, typeBase, &dirs)
		}

		// Collect using repo topic dirs
		if using, ok := section["using"].(map[string]any); ok {
			collectRepoTopicDirs(using, typeBase, ev.File, &dirs, true)
		}

		// Collect repo topic dirs
		if repos, ok := section["repo"].([]any); ok {
			for _, r := range repos {
				if repo, ok := r.(map[string]any); ok {
					collectRepoTopicDirs(repo, typeBase, ev.File, &dirs, false)
				}
			}
		}

		return nil
	})

	return dirs, err
}

func collectTopicDirs(file string, topics []any, base string, dirs *[]string) {
	for _, t := range topics {
		topic, ok := t.(map[string]any)
		if !ok {
			continue
		}

		topicDirName := getTopicDirName(topic)
		if topicDirName == "" {
			continue
		}

		topicBase := base + "/" + topicDirName

		if topicHasPicture(topic) {
			*dirs = append(*dirs, topicBase)
		}

		if subs, ok := topic["sub"].([]any); ok {
			collectTopicDirs(file, subs, topicBase, dirs)
		}
	}
}

func collectRepoTopicDirs(repo map[string]any, base, file string, dirs *[]string, useBase bool) {
	urlStr, _ := repo["url"].(string)
	repoName := repoNameFromURL(urlStr)
	if repoName == "" {
		return
	}

	repoBase := base + "/" + repoName
	topicBase := repoBase
	if useBase {
		topicBase = base
	}

	if topics, ok := repo["topics"].([]any); ok {
		collectTopicDirs(file, topics, topicBase, dirs)
	}
}

func getTopicDirName(topic map[string]any) string {
	if meta, ok := topic["meta"].(map[string]any); ok {
		if slug, ok := meta["slug"].(string); ok && slug != "" {
			return slug
		}
	}
	if t, ok := topic["topic"].(string); ok && t != "" {
		return t
	}

	return ""
}

func topicHasPicture(topic map[string]any) bool {
	if meta, ok := topic["meta"].(map[string]any); ok {
		if hp, ok := meta["hasPic"].(bool); ok && hp {
			return true
		}
	}
	hp, _ := topic["hasPic"].(bool)

	return hp
}

func repoNameFromURL(urlStr string) string {
	return urlutil.RepoName(urlStr)
}

// collectExistingFilesAndDirs returns both directories and files in imagesDir.
//
//nolint:nonamedreturns // named returns preferred for readability here
func collectExistingFilesAndDirs(imagesDir string) (dirs, files []string, err error) {
	err = filepath.WalkDir(imagesDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == imagesDir {
			return nil
		}
		rel, _ := filepath.Rel(imagesDir, path)
		normalized := strings.ReplaceAll(rel, string(filepath.Separator), "/")
		if d.IsDir() {
			dirs = append(dirs, normalized)
		} else {
			files = append(files, normalized)
		}

		return nil
	})

	return dirs, files, err
}

// Issues returns the check result as common checkutil issues.
func (r *CheckResult) Issues(cfg CheckConfig) []checkutil.Issue {
	var issues []checkutil.Issue
	for _, w := range r.Warnings {
		issues = append(issues, checkutil.Issue{
			File: "images", Severity: checkutil.SeverityWarn, Message: w,
		})
	}
	for _, e := range r.Errors {
		issues = append(issues, checkutil.Issue{
			File: "images", Severity: checkutil.SeverityError, Message: e,
		})
	}
	for _, d := range r.DuplicateFiles {
		issues = append(issues, checkutil.Issue{
			File: d, Severity: checkutil.SeverityWarn, Message: "duplicate image file",
		})
	}
	if !cfg.SkipMissing {
		for _, d := range r.MissingDirs {
			issues = append(issues, checkutil.Issue{
				File: d, Severity: checkutil.SeverityError, Message: "missing expected image dir",
			})
		}
	}
	if !cfg.SkipExtra {
		for _, d := range r.ExtraDirs {
			issues = append(issues, checkutil.Issue{
				File: d, Severity: checkutil.SeverityWarn, Message: "extra image dir",
			})
		}
	}

	return issues
}

// ReportResult returns the formatted check result.
func (r *CheckResult) ReportResult(cfg CheckConfig) string {
	var out strings.Builder

	checkResult := &checkutil.Result{Issues: r.Issues(cfg)}
	out.WriteString(checkResult.ReportResult("images check"))

	if cfg.List {
		for _, d := range r.ExpectedDirs {
			fmt.Fprintf(&out, "expected: %s\n", d)
		}
		for _, d := range r.ExistingDirs {
			fmt.Fprintf(&out, "existing: %s\n", d)
		}
	}

	if cfg.Apply && len(r.ApplyActions) > 0 {
		out.WriteString("\n[apply]\n")
		for _, a := range r.ApplyActions {
			fmt.Fprintf(&out, "  %s\n", a)
		}
	}

	return out.String()
}
