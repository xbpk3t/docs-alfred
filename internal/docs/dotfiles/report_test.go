package dotfiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// --- FormatText ---

func TestFormatText_NoIssues(t *testing.T) {
	r := &DiffResult{
		Category: CatDiff{Shared: []string{"a"}},
		Nix:      NixDiff{Shared: 1},
	}
	s := FormatText(r)
	assert.Contains(t, s, "✅ dotfiles check passed")
	assert.Contains(t, s, "categories shared=1")
}

func TestFormatText_WithErrors(t *testing.T) {
	r := &DiffResult{
		Issues: []checkutil.Issue{
			{File: "data/gh", Severity: checkutil.SeverityError, Message: "gh-only: curl"},
		},
		Category: CatDiff{GhOnly: []string{"utils"}},
		Nix:      NixDiff{GhOnly: map[string][]string{"curl": {"utils"}}},
	}
	s := FormatText(r)
	assert.Contains(t, s, "❌ dotfiles check failed")
	assert.Contains(t, s, "gh-only: curl")
}

func TestFormatText_WithWarnings(t *testing.T) {
	r := &DiffResult{
		Issues: []checkutil.Issue{
			{File: "category:a", Severity: checkutil.SeverityWarn, Message: "warning"},
		},
	}
	s := FormatText(r)
	assert.Contains(t, s, "✅ dotfiles check passed (with warnings)")
}

// --- FormatJSON ---

func TestFormatJSON_OK(t *testing.T) {
	r := &DiffResult{
		Summary: map[string]any{"shared": 1},
	}
	j := FormatJSON(r)
	assert.Equal(t, "dotfiles check", j["name"])
	assert.Equal(t, true, j["ok"])
	assert.NotNil(t, j["summary"])
}

func TestFormatJSON_HasErrors(t *testing.T) {
	r := &DiffResult{
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityError, Message: "bad"},
		},
	}
	j := FormatJSON(r)
	assert.Equal(t, false, j["ok"])
}

// --- FormatDedupText ---

func TestFormatDedupText_Empty(t *testing.T) {
	s := FormatDedupText(nil)
	assert.Equal(t, "no duplicates found\n", s)
}

func TestFormatDedupText_WithDups(t *testing.T) {
	dups := map[string][]string{
		"curl": {"cat1", "cat2"},
	}
	s := FormatDedupText(dups)
	assert.Contains(t, s, "found 1 duplicate package references")
	assert.Contains(t, s, "pkgs.curl referenced in multiple categories: cat1, cat2")
}

// --- FormatDedupJSON ---

func TestFormatDedupJSON_Empty(t *testing.T) {
	j := FormatDedupJSON(nil)
	assert.Equal(t, "dotfiles dedup", j["name"])
	assert.Equal(t, true, j["ok"])
	assert.Equal(t, 0, j["total"])
}

func TestFormatDedupJSON_WithDups(t *testing.T) {
	dups := map[string][]string{
		"curl": {"cat1", "cat2"},
	}
	j := FormatDedupJSON(dups)
	assert.Equal(t, "dotfiles dedup", j["name"])
	assert.Equal(t, true, j["ok"])
	assert.Equal(t, 1, j["total"])
	results := j["results"].([]checkutil.Issue)
	assert.Len(t, results, 1)
	assert.Contains(t, results[0].Message, "pkgs.curl")
}

// --- FormatCompact ---

func TestFormatCompact_AllZero(t *testing.T) {
	r := &DiffResult{}
	s := FormatCompact(r)
	assert.Contains(t, s, "categories shared=0 df-only=0 gh-only=0")
	assert.Contains(t, s, "nix gh-only=0 df-only=0")
}
