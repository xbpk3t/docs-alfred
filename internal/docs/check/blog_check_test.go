package workspaceops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunBlogCheckOKF(t *testing.T) {
	cases := []struct {
		name       string
		rel        string
		content    string
		wantIssues int
		checkMsgs  []string
	}{
		{
			name: "valid blog post passes",
			rel:  "AI/LLM-use/2026-04-01-superpowers-evaluation.md",
			content: `---
title: T
date: 2026-04-01
source: src
type: blog
---
body
`,
			wantIssues: 0,
		},
		{
			name: "blog post with non-blog type rejected",
			rel:  "AI/LLM-use/2026-04-01-post.md",
			content: `---
title: T
date: 2026-04-01
source: src
type: research
---
body
`,
			wantIssues: 1,
			checkMsgs:  []string{"must have type=blog"},
		},
		{
			name: "missing required fields rejected",
			rel:  "db/RDB/post.md",
			content: `---
type: blog
---
body
`,
			wantIssues: 2,
			checkMsgs:  []string{"missing required field: title", "missing required field: date"},
		},
		{
			name: "public build artifacts skipped",
			rel:  "public/redirects.json",
			content: "{}",
			wantIssues: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, tt.rel, tt.content)

			issues, err := RunBlogCheckOKF(root)
			require.NoError(t, err)
			require.Len(t, issues, tt.wantIssues, "expected %d issues, got %d: %+v", tt.wantIssues, len(issues), issues)
			for _, msg := range tt.checkMsgs {
				assertIssueContains(t, issues, msg)
			}
		})
	}
}
