// Package gitutil provides go-git based utilities for common git operations,
// replacing shell-out calls to the git CLI.
//
// Supported operations:
//   - Porcelain status (git status --porcelain)
//   - Changed file listing
//   - Diff stat (git diff --stat)
//   - Repo root detection (git rev-parse --show-toplevel)
package gitutil

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// ChangedFile represents a single file change from git status.
type ChangedFile struct {
	Path   string
	Status string // two-char porcelain status, e.g. " M", "??", "A "
}

// PorcelainStatus returns the git status in porcelain format (equivalent to
// `git status --porcelain`). Each line is "XY path" where X is staging status
// and Y is worktree status.
func PorcelainStatus(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo %s: %w", repoPath, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("status: %w", err)
	}

	return status.String(), nil
}

// ChangedFiles returns the list of changed files in the repository.
// Each file has a two-character status string matching git porcelain format.
func ChangedFiles(repoPath string) ([]ChangedFile, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo %s: %w", repoPath, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	var files []ChangedFile
	for path, fs := range status {
		if fs.Staging == git.Unmodified && fs.Worktree == git.Unmodified {
			continue
		}
		staging := byte(fs.Staging)
		worktree := byte(fs.Worktree)
		statusStr := string([]byte{staging, worktree})
		files = append(files, ChangedFile{Path: path, Status: statusStr})
	}

	return files, nil
}

// DiffStat returns a git diff --stat formatted string for a single file,
// comparing the HEAD version against the current worktree version.
// Returns empty string if the file is unchanged or not tracked.
func DiffStat(repoPath, filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	repoRoot, err := FindRepoRoot(repoPath)
	if err != nil {
		return "", err
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return "", fmt.Errorf("open repo %s: %w", repoPath, err)
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("relative path: %w", err)
	}
	relPath = filepath.ToSlash(relPath)

	// Get HEAD file content
	headContent, err := headFileContent(repo, relPath)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			return "", nil // new/untracked file — no diff
		}

		return "", fmt.Errorf("read HEAD file: %w", err)
	}

	// Get worktree file content
	workContent, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read worktree file: %w", err)
	}

	additions, deletions := countLineDiff(headContent, string(workContent))
	total := additions + deletions
	if total == 0 {
		return "", nil
	}

	// Format: " path/to/file.go | N +++---"
	changeStr := strconv.Itoa(total)
	graph := strings.Repeat("+", additions) + strings.Repeat("-", deletions)

	return fmt.Sprintf(" %s | %s %s", relPath, changeStr, graph), nil
}

// FindRepoRoot returns the root directory of the git repository containing
// the given path. Equivalent to `git rev-parse --show-toplevel`.
func FindRepoRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Walk up from absPath to find a .git directory
	dir := absPath
	for {
		gitDir := filepath.Join(dir, ".git")
		info, statErr := os.Stat(gitDir)
		if statErr == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no git repository found from %s", path)
}

// headFileContent reads a file's content from the HEAD commit.
func headFileContent(repo *git.Repository, relPath string) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	commit, err := object.GetCommit(repo.Storer, head.Hash())
	if err != nil {
		return "", err
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	f, err := tree.File(relPath)
	if err != nil {
		return "", err
	}

	contents, err := f.Contents()
	if err != nil {
		return "", err
	}

	return contents, nil
}

// countLineDiff counts additions and deletions between old and new content
// using a simple LCS-based diff. Returns (additions, deletions).
func countLineDiff(old, newContent string) (int, int) {
	oldLines := splitLines(old)
	newLines := splitLines(newContent)
	dp := buildLCSTable(oldLines, newLines)

	return backtrackDiff(dp, oldLines, newLines)
}

// buildLCSTable builds the LCS dynamic programming table.
func buildLCSTable(oldLines, newLines []string) [][]int {
	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	return dp
}

// backtrackDiff walks the LCS table to count additions and deletions.
func backtrackDiff(dp [][]int, oldLines, newLines []string) (int, int) {
	var additions, deletions int
	i, j := len(oldLines), len(newLines)
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			additions++
			j--
		} else {
			deletions++
			i--
		}
	}

	return additions, deletions
}

func splitLines(s string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}
