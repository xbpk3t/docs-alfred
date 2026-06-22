package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func changedWikiMarkdownPaths(ctx context.Context, wikiRoot string, runCmd CommandRunner) ([]string, error) {
	roots, err := changedWikiGitRoots(ctx, wikiRoot, runCmd)
	if err != nil {
		return nil, err
	}

	return changedMarkdownPathsFromGit(ctx, roots.repoRoot, roots.relWikiRoot, runCmd)
}

type changedWikiRoots struct {
	repoRoot    string
	relWikiRoot string
}

func changedWikiGitRoots(ctx context.Context, wikiRoot string, runCmd CommandRunner) (changedWikiRoots, error) {
	absWikiRoot, err := filepath.Abs(wikiRoot)
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("resolve wiki root: %w", err)
	}
	absWikiRoot = evalSymlinksOrOriginal(absWikiRoot)
	repoRoot, err := gitOutput(ctx, absWikiRoot, runCmd, "rev-parse", "--show-toplevel")
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("find git worktree for changed-only audit: %w", err)
	}
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return changedWikiRoots{}, errors.New("find git worktree for changed-only audit: empty git root")
	}
	repoRoot = evalSymlinksOrOriginal(repoRoot)
	relWikiRoot, err := filepath.Rel(repoRoot, absWikiRoot)
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("resolve wiki root relative to git root: %w", err)
	}

	return changedWikiRoots{repoRoot: repoRoot, relWikiRoot: filepath.ToSlash(relWikiRoot)}, nil
}

func evalSymlinksOrOriginal(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	return resolved
}

func changedMarkdownPathsFromGit(ctx context.Context, repoRoot, relWikiRoot string, runCmd CommandRunner) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	for _, args := range [][]string{
		{"diff", "--name-only", "--cached", "--", relWikiRoot},
		{"diff", "--name-only", "--", relWikiRoot},
		{"ls-files", "--others", "--exclude-standard", "--", relWikiRoot},
	} {
		out, err := gitOutput(ctx, repoRoot, runCmd, args...)
		if err != nil {
			return nil, fmt.Errorf("list changed wiki files: %w", err)
		}
		paths = appendChangedMarkdownPaths(paths, seen, repoRoot, out)
	}

	return paths, nil
}

func appendChangedMarkdownPaths(paths []string, seen map[string]bool, repoRoot, output string) []string {
	for rel := range strings.FieldsSeq(output) {
		if filepath.Ext(rel) != ".md" {
			continue
		}
		path := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if seen[path] || !fileExists(path) {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}

	return paths
}

func gitOutput(ctx context.Context, dir string, runCmd CommandRunner, args ...string) (string, error) {
	if runCmd == nil {
		return "", fmt.Errorf("git %s: CommandRunner not provided", strings.Join(args, " "))
	}
	out, err := runCmd(ctx, dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}
