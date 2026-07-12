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
	frontmatter, err := generateFrontmatter("Test Title", SourceCodex)

	require.NoError(t, err)
	require.Contains(t, frontmatter, "source: codex")
}

func TestFallbackExportMetadata_UsesFallbackTopic(t *testing.T) {
	messages := []session.Message{{Role: "user", Content: "请帮我整理 codex export\n\n后续内容"}}

	topicPath, title, engTitle := fallbackExportMetadata(messages)

	require.Equal(t, fallbackTopicPath, topicPath)
	require.Equal(t, "请帮我整理 codex export", title)
	require.Equal(t, "qing-bang-wo-zheng-li-codex-export", engTitle)
}

func TestNormalizeTopicPath(t *testing.T) {
	wikiRoot := t.TempDir()
	candidates := []ghindex.TopicCandidate{
		{Path: "AI/LLM/claude-code"},
		{Path: fallbackTopicPath},
	}

	tests := []struct {
		name      string
		topicPath string
		want      string
	}{
		{
			name:      "candidate path",
			topicPath: "AI/LLM/claude-code",
			want:      "AI/LLM/claude-code",
		},
		{
			name:      "empty path",
			topicPath: "",
			want:      fallbackTopicPath,
		},
		{
			name:      "none path",
			topicPath: "none",
			want:      fallbackTopicPath,
		},
		{
			name:      "inbox path",
			topicPath: "inbox",
			want:      fallbackTopicPath,
		},
		{
			name:      "unknown path",
			topicPath: "AI/LLM/missing",
			want:      fallbackTopicPath,
		},
		{
			name:      "unsafe path",
			topicPath: "../escape",
			want:      fallbackTopicPath,
		},
		{
			name:      "wrong depth",
			topicPath: "AI/LLM",
			want:      fallbackTopicPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTopicPath(wikiRoot, tt.topicPath, candidates)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestSelectMessages_Overlap(t *testing.T) {
	tests := []struct {
		name   string
		msgs   []string
		head   int
		tail   int
		want   []string
	}{
		{
			name: "no overlap",
			msgs: []string{"a", "b", "c", "d", "e"},
			head: 2,
			tail: 2,
			want: []string{"a", "b", "d", "e"},
		},
		{
			name: "exact fit returns all",
			msgs: []string{"a", "b", "c"},
			head: 2,
			tail: 1,
			want: []string{"a", "b", "c"},
		},
		{
			name: "head+tail exceeds len returns all without dup",
			msgs: []string{"a", "b", "c"},
			head: 3,
			tail: 3,
			want: []string{"a", "b", "c"},
		},
		{
			name: "head negative clamped to zero",
			msgs: []string{"a", "b", "c", "d"},
			head: -1,
			tail: 2,
			want: []string{"c", "d"},
		},
		{
			name: "single element",
			msgs: []string{"only"},
			head: 1,
			tail: 1,
			want: []string{"only"},
		},
		{
			name: "tail zero",
			msgs: []string{"a", "b", "c"},
			head: 2,
			tail: 0,
			want: []string{"a", "b"},
		},
		{
			name: "tail negative clamped to zero",
			msgs: []string{"a", "b", "c", "d"},
			head: 2,
			tail: -1,
			want: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectMessages(tt.msgs, tt.head, tt.tail)
			require.Equal(t, tt.want, got)
		})
	}
}
