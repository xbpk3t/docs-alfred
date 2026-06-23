package presenter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func TestBadgeIconPathJoinsCacheDir(t *testing.T) {
	dir := useTempRepoIconCache(t)
	got := badgeIconPath(badgeState{HasDoc: true, HasNix: false, Score: 3})
	assert.Equal(t, filepath.Join(dir, "gh-d1-n0-s3.svg"), got)
}

func TestBoolInt(t *testing.T) {
	assert.Equal(t, 1, boolInt(true))
	assert.Equal(t, 0, boolInt(false))
}

func TestClampScore(t *testing.T) {
	assert.Equal(t, 0, clampScore(-10))
	assert.Equal(t, 0, clampScore(0))
	assert.Equal(t, 3, clampScore(3))
	assert.Equal(t, 5, clampScore(5))
	assert.Equal(t, 5, clampScore(100))
}

func TestBadgeFillActiveAndInactive(t *testing.T) {
	assert.Equal(t, "#F97316", badgeFill(true, "#F97316"))
	assert.Equal(t, "#4B5563", badgeFill(false, "#F97316"))
}

func TestBadgeTextActiveAndInactive(t *testing.T) {
	assert.Equal(t, "#FFFFFF", badgeText(true, "#FFFFFF"))
	assert.Equal(t, "#D1D5DB", badgeText(false, "#FFFFFF"))
}

func TestDocBadgeSVGActiveAndInactive(t *testing.T) {
	active := docBadgeSVG(true)
	assert.Contains(t, active, ">D</text>")
	assert.Contains(t, active, "#F97316")

	inactive := docBadgeSVG(false)
	assert.Contains(t, inactive, ">D</text>")
	assert.Contains(t, inactive, "#4B5563")
}

func TestNixBadgeSVGActiveAndInactive(t *testing.T) {
	active := nixBadgeSVG(true)
	assert.Contains(t, active, ">N</text>")
	assert.Contains(t, active, "#DC2626")

	inactive := nixBadgeSVG(false)
	assert.Contains(t, inactive, ">N</text>")
	assert.Contains(t, inactive, "#4B5563")
}

func TestScoreBadgeSVGValues(t *testing.T) {
	for score := 0; score <= 5; score++ {
		svg := scoreBadgeSVG(score)
		assert.Contains(t, svg, "#FABC05")
	}
}

func TestRenderBadgeIconSVGContainsCheckMark(t *testing.T) {
	svg := renderBadgeIconSVG(badgeState{HasDoc: true, HasNix: true, Score: 3})
	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, ">D</text>")
	assert.Contains(t, svg, ">N</text>")
	assert.Contains(t, svg, ">3</text>")
	assert.Contains(t, svg, "#77B255")
}

func TestRepoIconPathNilRepo(t *testing.T) {
	assert.Equal(t, IconGh, repoIconPath(nil))
}

func TestEnsureBadgeIconCachesSVG(t *testing.T) {
	useTempRepoIconCache(t)
	state := badgeState{HasDoc: true, HasNix: false, Score: 2}

	path1, ok1 := ensureBadgeIcon(state)
	require.True(t, ok1)

	path2, ok2 := ensureBadgeIcon(state)
	require.True(t, ok2)
	assert.Equal(t, path1, path2)
}

func TestRepoBadgeStateWithNixURL(t *testing.T) {
	repo := &ghindex.Repository{
		NixURL: "github:acme/tool#tool",
		Score:  3,
	}
	got := repoBadgeState(repo)
	assert.True(t, got.HasNix)
	assert.Equal(t, 3, got.Score)
}
