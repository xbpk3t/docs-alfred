package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
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

func TestNormalizeTopicPath(t *testing.T) {
	wikiRoot := t.TempDir()
	candidates := []ghindex.TopicCandidate{
		{Path: "AI/LLM/claude-code"},
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
			topicPath: "AI/LLM/claude-code",
			wantOK:    "AI/LLM/claude-code",
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
