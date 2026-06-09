package presenter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

func TestRepoBadgeState(t *testing.T) {
	tests := []struct {
		name string
		repo *ghindex.Repository
		want badgeState
	}{
		{name: "nil", repo: nil, want: badgeState{}},
		{name: "empty", repo: &ghindex.Repository{}, want: badgeState{Score: 0}},
		{name: "doc nix score", repo: &ghindex.Repository{Doc: "data/gh/tool", NixURL: "github:acme/tool#tool", Score: 4}, want: badgeState{HasDoc: true, HasNix: true, Score: 4}},
		{name: "blank doc nix", repo: &ghindex.Repository{Doc: " ", NixURL: " ", Score: 2}, want: badgeState{Score: 2}},
		{name: "negative score", repo: &ghindex.Repository{Score: -1}, want: badgeState{Score: 0}},
		{name: "over max score", repo: &ghindex.Repository{Score: 9}, want: badgeState{Score: 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, repoBadgeState(tt.repo))
		})
	}
}

func TestBadgeIconNameIsStable(t *testing.T) {
	assert.Equal(t, "gh-d1-n0-s5.svg", badgeIconName(badgeState{HasDoc: true, Score: 8}))
	assert.Equal(t, "gh-d0-n1-s0.svg", badgeIconName(badgeState{HasNix: true, Score: -1}))
}

func TestEnsureBadgeIconWritesSVG(t *testing.T) {
	cacheDir := useTempRepoIconCache(t)
	state := badgeState{HasDoc: true, HasNix: true, Score: 5}

	path, ok := ensureBadgeIcon(state)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(cacheDir, "gh-d1-n1-s5.svg"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	svg := string(data)
	assert.True(t, strings.Contains(svg, ">D</text>"))
	assert.True(t, strings.Contains(svg, ">N</text>"))
	assert.True(t, strings.Contains(svg, ">5</text>"))
}

func TestRepoIconPathFallsBackWhenGenerationFails(t *testing.T) {
	old := repoIconCacheDir
	notDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(notDir, []byte("x"), fileutil.FilePermPrivate))
	repoIconCacheDir = notDir
	t.Cleanup(func() {
		repoIconCacheDir = old
	})

	got := repoIconPath(&ghindex.Repository{Doc: "data/gh/tool", NixURL: "github:acme/tool#tool", Score: 5})
	assert.Equal(t, IconGh, got)
}
