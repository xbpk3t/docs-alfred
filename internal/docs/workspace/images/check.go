package images

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/data"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
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
	result.DuplicateFiles = findDuplicateFiles(actualFiles)

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

	// Find extra — skip structural intermediate dirs
	for _, d := range existingDirs {
		if expectedSet[d] {
			continue
		}
		// Structural intermediate dir (parent of an expected dir with no direct files)
		// is silently accepted — it's a container for deeper expected subdirs.
		if isParentOfExpected(d, expectedDirs) && !dirHasDirectFiles(d, actualFiles) {
			continue
		}
		result.ExtraDirs = append(result.ExtraDirs, d)
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

func findDuplicateFiles(actualFiles []string) []string {
	actualSet := make(map[string]bool, len(actualFiles))
	for _, f := range actualFiles {
		actualSet[f] = true
	}

	var duplicates []string
	for _, f := range actualFiles {
		base := filepath.Base(f)
		matches := duplicateFileRe.FindStringSubmatch(base)
		if matches == nil {
			continue
		}

		originalName := matches[1] + matches[2]
		dir := filepath.Dir(f)
		originalRel := originalName
		if dir != "." {
			originalRel = filepath.ToSlash(filepath.Join(dir, originalName))
		}
		if actualSet[originalRel] {
			duplicates = append(duplicates, f)
		}
	}

	return duplicates
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

// isParentOfExpected reports whether dir is a parent of any expected directory,
// meaning it exists only as a structural container for deeper expected subdirs.
//
// The trailing-slash prefix check ensures "a-new" is NOT treated as a parent
// of "a".
func isParentOfExpected(dir string, expectedDirs []string) bool {
	prefix := dir + "/"
	for _, ed := range expectedDirs {
		if strings.HasPrefix(ed, prefix) {
			return true
		}
	}
	return false
}

// dirHasDirectFiles reports whether dir contains files directly (not in subdirs).
// Used to detect "files at wrong layer" — a structural dir with direct files needs attention.
//
// allFiles must be the same filtered list used by collectExistingFilesAndDirs
// (which skips dot-prefixed entries). This implicitly excludes .DS_Store and
// other hidden files from the check — an intentional property.
//
// Performance note: O(n×m) scan of all files per dir. Fine for current scale
// (~200 dirs × ~800 files). If scale grows significantly, consider building a
// prefix-indexed set for O(1) lookups.
func dirHasDirectFiles(dir string, allFiles []string) bool {
	prefix := dir + "/"
	for _, f := range allFiles {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := strings.TrimPrefix(f, prefix)
		if !strings.Contains(rest, "/") {
			return true
		}
	}
	return false
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
	_ = fileutil.EnsureDir(tempDir)

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

	err := ghdata.WalkGhRepos(dataDir, func(ev ghdata.WalkerEvent) error {
		if ev.Type != "section" {
			return nil
		}

		section := ev.Section
		typeVal := section.Type
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
		collectTopicDirs(section.Topics, typeBase, &dirs)

		// Collect repo topic dirs
		for i := range section.Repos {
			collectRepoTopicDirs(&section.Repos[i], typeBase, &dirs, false)
		}

		return nil
	})

	return dirs, err
}

func collectTopicDirs(topics []ghdata.Topic, base string, dirs *[]string) {
	for i := range topics {
		topic := topics[i]
		topicDirName := topic.DirName()
		if topicDirName == "" {
			continue
		}

		topicBase := base + "/" + topicDirName

		*dirs = append(*dirs, topicBase)

		// 收集 topic 内嵌 repo 的图片目录
		for _, repo := range topic.Repos {
			repoName := urlutil.RepoName(repo.URL)
			if repoName == "" {
				continue
			}
			repoBase := topicBase + "/" + repoName
			// 收集 repo 的 topics 的图片目录
			collectTopicDirs(repo.Topics, repoBase, dirs)
		}

		// 递归处理子 topics
		collectTopicDirs(topic.Sub, topicBase, dirs)
	}
}

func collectRepoTopicDirs(repo *ghdata.Repo, base string, dirs *[]string, useBase bool) {
	urlStr := repo.URL
	repoName := urlutil.RepoName(urlStr)
	if repoName == "" {
		return
	}

	repoBase := base + "/" + repoName
	topicBase := repoBase
	if useBase {
		topicBase = base
	}

	collectTopicDirs(repo.Topics, topicBase, dirs)
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
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

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
