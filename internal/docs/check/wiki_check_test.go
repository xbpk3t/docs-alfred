package workspaceops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestRunWikiCheckOKF(t *testing.T) {
	tests := []struct {
		name       string
		rel        string
		content    string
		checkMsgs  []string
		wantIssues int
	}{
		{
			name: "valid minimal frontmatter at topic level",
			rel:  "folder/type/topic/test.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: session
---
body
`,
			wantIssues: 0,
		},
		{
			name: "optional session and model fields allowed",
			rel:  "folder/type/topic/with-session.md",
			content: `---
title: T
date: 2026-06-17
source: claude-code
type: research
session: fbeb823d-4f1f-4213-8c67-e480e089a1a0
model: grok-4.5
---
body
`,
			wantIssues: 0,
		},
		{
			name: "optional session without model allowed",
			rel:  "folder/type/topic/session-only.md",
			content: `---
title: T
date: 2026-06-17
source: codex
type: research
session: 019f26c6-d9e4-7760-854d-ee8fcf8ed03b
---
body
`,
			wantIssues: 0,
		},
		{
			name: "optional issue field allowed",
			rel:  "folder/type/topic/with-issue.md",
			content: `---
title: T
date: 2026-06-17
source: claude-code
type: research
session: e0ebe705-e20e-4130-a6a7-9a2e719b6841
model: grok-4.5
issue: https://linear.app/luckzzz/issue/LUC-284/x
---
body
`,
			wantIssues: 0,
		},
		{
			name: "optional score field allowed",
			rel:  "folder/type/topic/with-score.md",
			content: `---
title: T
date: 2026-06-17
source: claude-code
type: research
session: e0ebe705-e20e-4130-a6a7-9a2e719b6841
model: grok-4.5
score: 0
---
body
`,
			wantIssues: 0,
		},
		{
			name: "optional issue and project fields allowed",
			rel:  "folder/type/topic/with-issue.md",
			content: `---
title: T
date: 2026-06-17
source: claude-code
type: research
session: abc
model: grok-4.5
issue: https://linear.app/luckzzz/issue/LUC-279
project: docs
---
body
`,
			wantIssues: 0,
		},
		{
			name: "no frontmatter at all",
			rel:  "folder/type/topic/plain.md",
			content: `hello world

some content
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing frontmatter"},
		},
		{
			name: "single opener only",
			rel:  "folder/type/topic/no-close.md",
			content: `---

content without closing
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing frontmatter"},
		},
		{
			name: "missing type field",
			rel:  "folder/type/topic/no-type.md",
			content: `---
title: T
date: 2026-06-17
source: src
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing required field: type"},
		},
		{
			name: "invalid type value",
			rel:  "folder/type/topic/bad-type.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: foobar
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"invalid OKF type: foobar"},
		},
		{
			name: "missing title",
			rel:  "folder/type/topic/no-title.md",
			content: `---
title: ""
date: 2026-06-17
source: src
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing required field: title"},
		},
		{
			name: "missing date no field",
			rel:  "folder/type/topic/no-date.md",
			content: `---
title: T
source: src
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing required field: date"},
		},
		{
			name: "bad date format",
			rel:  "folder/type/topic/bad-date.md",
			content: `---
title: T
date: "2026/03/30"
source: src
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"invalid date format"},
		},
		{
			name: "missing source",
			rel:  "folder/type/topic/no-source.md",
			content: `---
title: T
date: 2026-06-17
source: ""
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing required field: source"},
		},
		{
			name: "source empty YAML null",
			rel:  "folder/type/topic/null-source.md",
			content: `---
title: T
date: 2026-06-17
source:
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"missing required field: source"},
		},
		{
			name: "bare root file skipped",
			rel:  "test.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: session
---
body
`,
			wantIssues: 0,
		},
		{
			name: "depth-1 file skipped",
			rel:  "zzz/test.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: session
---
body
`,
			wantIssues: 0,
		},
		{
			name: "empty frontmatter four missing fields",
			rel:  "folder/type/topic/empty-fm.md",
			content: `---
---
`,
			wantIssues: 4,
			checkMsgs:  []string{"title", "date", "source", "type"},
		},
		{
			name: "stray depth-2 file flagged",
			rel:  "folder/type/stray.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: session
---
body
`,
			wantIssues: 1,
			checkMsgs:  []string{"stray .md file at type level"},
		},
		{
			name: "walk error unreadable file",
			rel:  "folder/type/topic/unreadable.md",
			content: `---
title: T
date: 2026-06-17
source: src
type: session
---
`,
			wantIssues: 1,
			checkMsgs:  []string{"read error"},
		},
	}

	// Build the set of all OKF valid types for the "all 9 types" test.
	allTypeCases := []struct {
		name string
		typ  string
	}{
		{"type session", "session"},
		{"type review", "review"},
		{"type blog", "blog"},
		{"type log", "log"},
		{"type digest", "digest"},
		{"type reference", "reference"},
		{"type research", "research"},
		{"type transcript", "transcript"},
		{"type queue", "queue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, tt.rel, tt.content)

			// For the unreadable test case, remove read permissions.
			if strings.Contains(tt.name, "unreadable") {
				fullPath := filepath.Join(root, tt.rel)
				require.NoError(t, os.Chmod(fullPath, 0o000))
				t.Cleanup(func() { _ = os.Chmod(fullPath, 0o600) })
			}

			issues, err := RunWikiCheckOKF(root)
			require.NoError(t, err)
			require.Len(t, issues, tt.wantIssues, "expected %d issues, got %d: %+v", tt.wantIssues, len(issues), issues)
			for _, msg := range tt.checkMsgs {
				assertIssueContains(t, issues, msg)
			}
		})
	}

	// Test all 9 valid types produce 0 issues each.
	t.Run("valid all 9 types", func(t *testing.T) {
		root := t.TempDir()
		for _, tc := range allTypeCases {
			content := "---\ntitle: T\ndate: 2026-06-17\nsource: src\ntype: " + tc.typ + "\n---\nbody\n"
			writeTestFile(t, root, "folder/type/topic/"+tc.typ+".md", content)
		}
		issues, err := RunWikiCheckOKF(root)
		require.NoError(t, err)
		require.Empty(t, issues, "all 9 valid types should produce 0 issues, got: %+v", issues)
	})
}

// writeTestFile writes content to a file at rel under root, creating directories as needed.
func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	fullPath := filepath.Join(root, rel)
	dir := filepath.Dir(fullPath)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
}

// assertIssueContains checks that at least one issue message contains the substring.
func assertIssueContains(t *testing.T, issues []checkutil.Issue, substr string) {
	t.Helper()
	for _, issue := range issues {
		if strings.Contains(issue.Message, substr) {
			return
		}
	}
	// Collect all messages for a helpful failure message.
	var msgs []string
	for _, issue := range issues {
		msgs = append(msgs, issue.Message)
	}
	assert.Fail(t, "expected issue containing %q, got: %v", substr, msgs)
}

// --- addDir duplicate path ---

func TestAddDirDuplicate(t *testing.T) {
	set := map[string]bool{"a": true}
	var dirs []string
	addDir("a", set, &dirs)
	assert.Empty(t, dirs) // already in set, not added
}

func TestAddDirNew(t *testing.T) {
	set := map[string]bool{}
	var dirs []string
	addDir("b", set, &dirs)
	assert.Len(t, dirs, 1)
	assert.True(t, set["b"])
}

// --- slashRel ---

func TestSlashRel(t *testing.T) {
	result := slashRel("/some/root", "/some/root/path/file.md")
	assert.Equal(t, "path/file.md", result)
}

// --- collectWikiDepth2Dirs ---

func TestCollectWikiDepth2Dirs_NonExistent(t *testing.T) {
	root := t.TempDir()
	set := make(map[string]bool)
	var dirs []string
	collectWikiDepth2Dirs(root, "nonexistent", set, &dirs)
	assert.Empty(t, dirs)
}

func TestCollectWikiDepth2Dirs_WithHiddenSubdir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a", ".hidden"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a", "visible"), 0o700))

	set := make(map[string]bool)
	var dirs []string
	collectWikiDepth2Dirs(root, "a", set, &dirs)
	assert.Contains(t, dirs, "a/visible")
	assert.NotContains(t, dirs, "a/.hidden")
}

func TestCollectExpectedDirsInvalidPath(t *testing.T) {
	_, err := collectExpectedDirs("/tmp/nonexistent-gh-root-12345")
	require.Error(t, err)
}

func TestCollectActualWikiDirsInvalidPath(t *testing.T) {
	_, err := collectActualWikiDirs("/tmp/nonexistent-wiki-root-12345")
	require.Error(t, err)
}

func TestCollectActualWikiDirsSkipsFiles(t *testing.T) {
	root := t.TempDir()
	// Create a file (not a directory) at depth 1
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir1"), 0o700))

	dirs, err := collectActualWikiDirs(root)
	require.NoError(t, err)
	assert.Contains(t, dirs, "dir1")
	assert.NotContains(t, dirs, "file.txt")
}

func TestCollectWikiDepth2DirsSkipsFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "parent"), 0o700))
	// Create a file inside parent (not a dir)
	require.NoError(t, os.WriteFile(filepath.Join(root, "parent", "file.txt"), []byte("x"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "parent", "child"), 0o700))

	set := make(map[string]bool)
	var dirs []string
	collectWikiDepth2Dirs(root, "parent", set, &dirs)
	assert.Contains(t, dirs, "parent/child")
	assert.NotContains(t, dirs, "parent/file.txt")
}

func TestSlashRelError(t *testing.T) {
	// When filepath.Rel fails, slashRel falls back to filepath.ToSlash(path)
	// This happens when root and path are on different volumes (on Windows)
	// or when one is relative and the other absolute in certain cases.
	// On Unix, this is hard to trigger naturally, but we can test the normal path.
	result := slashRel("/a/b", "/a/b/c/d.md")
	assert.Equal(t, "c/d.md", result)
}

func TestCheckFileFrontmatterParseError(t *testing.T) {
	// Create a file with invalid frontmatter that causes parse error
	root := t.TempDir()
	// A file at depth 3 with content that frontmatter.Parse can't handle
	writeTestFile(t, root, "cat/type/topic/bad.md", "no frontmatter here")

	issues, err := RunWikiCheckOKF(root)
	require.NoError(t, err)
	// Should report "missing frontmatter" since there's no --- delimiters
	require.NotEmpty(t, issues)
	assertIssueContains(t, issues, "missing frontmatter")
}

func TestRunWikiCheckOKFEmptyRoot(t *testing.T) {
	root := t.TempDir()
	issues, err := RunWikiCheckOKF(root)
	require.NoError(t, err)
	assert.Empty(t, issues)
}
