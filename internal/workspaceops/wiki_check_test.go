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
		rel        string // relative path within wiki root
		content    string
		wantIssues int
		checkMsgs  []string // optional: specific messages that must appear
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
		{"type blog-draft", "blog-draft"},
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
