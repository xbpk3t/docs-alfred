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
