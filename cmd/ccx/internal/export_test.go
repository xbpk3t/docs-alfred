package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
	session "github.com/xbpk3t/docs-alfred/pkg/ai/session"
)

func TestTrimTitle_TruncatesRunes(t *testing.T) {
	title := strings.Repeat("中", 60)

	got := trimTitle(title)

	require.Len(t, []rune(got), 50)
	require.NotContains(t, got, "�")
}

func TestGenerateFrontmatter_UsesSource(t *testing.T) {
	frontmatter, err := generateFrontmatter("Test Title", SourceCodex, "thread-123", "gpt-5.5", "https://linear.app/x/issue/LUC-1")

	require.NoError(t, err)
	require.Contains(t, frontmatter, "source: codex")
	require.Contains(t, frontmatter, "session: thread-123")
	require.Contains(t, frontmatter, "model: gpt-5.5")
	require.Contains(t, frontmatter, "issue: https://linear.app/x/issue/LUC-1")
	require.Contains(t, frontmatter, "score: 0")
}

func TestGenerateFrontmatter_OmitsEmptyModel(t *testing.T) {
	frontmatter, err := generateFrontmatter("Test Title", SourceClaudeCode, "sess-abc", "", "")

	require.NoError(t, err)
	require.Contains(t, frontmatter, "session: sess-abc")
	require.NotContains(t, frontmatter, "model:")
	require.NotContains(t, frontmatter, "issue:")
	require.Contains(t, frontmatter, "score: 0")
}

func TestGenerateFrontmatter_OmitsEmptyIssue(t *testing.T) {
	frontmatter, err := generateFrontmatter("Test Title", SourceClaudeCode, "sess-abc", "grok-4.5", "")

	require.NoError(t, err)
	require.Contains(t, frontmatter, "model: grok-4.5")
	require.NotContains(t, frontmatter, "issue:")
	require.Contains(t, frontmatter, "score: 0")
}

func TestNormalizeTopicPath(t *testing.T) {
	wikiRoot := t.TempDir()
	candidates := []ghindex.TopicCandidate{
		{Path: "AI/LLM-use/claude-code"},
	}

	tests := []struct {
		name      string
		topicPath string
		wantOK    string
		wantErr   bool
		wantErrMsg string
	}{
		{
			name:      "valid candidate path",
			topicPath: "AI/LLM-use/claude-code",
			wantOK:    "AI/LLM-use/claude-code",
		},
		{
			name:      "empty path",
			topicPath: "",
			wantErr:   true,
		},
		{
			name:      "none path",
			topicPath: "none",
			wantErr:   true,
		},
		{
			name:      "inbox path",
			topicPath: "inbox",
			wantErr:   true,
		},
		{
			name:      "unknown path",
			topicPath: "AI/LLM/missing",
			wantErr:   true,
		},
		{
			name:      "unsafe path",
			topicPath: "../escape",
			wantErr:   true,
			wantErrMsg: "unsafe",
		},
		{
			name:      "wrong depth (too shallow)",
			topicPath: "AI/LLM",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTopicPath(wikiRoot, tt.topicPath, candidates)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErr && tt.wantErrMsg != "" {
				require.Contains(t, err.Error(), tt.wantErrMsg)
			} else if tt.wantErr {
				require.Error(t, err)
			}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantOK, got)
		})
	}
}

func TestExtractUserMessages(t *testing.T) {
	tests := []struct {
		name string
		msgs []session.Message
		want []string
	}{
		{
			name: "only user messages",
			msgs: []session.Message{
				{Role: "user", Content: "a"},
				{Role: "user", Content: "b"},
			},
			want: []string{"a", "b"},
		},
		{
			name: "mixed roles filter to user",
			msgs: []session.Message{
				{Role: "user", Content: "a"},
				{Role: "assistant", Content: "x"},
				{Role: "user", Content: "b"},
				{Role: "system", Content: "y"},
			},
			want: []string{"a", "b"},
		},
		{
			name: "no user messages",
			msgs: []session.Message{
				{Role: "assistant", Content: "x"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserMessages(tt.msgs)
			require.Equal(t, tt.want, got)
		})
	}
}
