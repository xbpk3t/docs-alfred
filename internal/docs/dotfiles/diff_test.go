package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// --- hasOverlap ---

func TestHasOverlap_Overlap(t *testing.T) {
	assert.True(t, hasOverlap([]string{"a", "b"}, []string{"b", "c"}))
}

func TestHasOverlap_NoOverlap(t *testing.T) {
	assert.False(t, hasOverlap([]string{"a", "b"}, []string{"c", "d"}))
}

func TestHasOverlap_Empty(t *testing.T) {
	assert.False(t, hasOverlap([]string{}, []string{"a"}))
	assert.False(t, hasOverlap([]string{"a"}, []string{}))
	assert.False(t, hasOverlap(nil, nil))
}

// --- DiffCategories ---

func TestDiffCategories_Shared(t *testing.T) {
	diff := DiffCategories([]string{"a", "b"}, []string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, diff.Shared)
	assert.Empty(t, diff.DfOnly)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.Issues)
}

func TestDiffCategories_DfOnly(t *testing.T) {
	diff := DiffCategories([]string{}, []string{"df1", "df2"})
	assert.Empty(t, diff.Shared)
	assert.Equal(t, []string{"df1", "df2"}, diff.DfOnly)
	assert.Empty(t, diff.GhOnly)
	assert.Len(t, diff.Issues, 2)
	for _, iss := range diff.Issues {
		assert.Equal(t, checkutil.SeverityError, iss.Severity)
		assert.Contains(t, iss.Message, "exists in dotfiles but not in data/gh")
	}
}

func TestDiffCategories_GhOnly(t *testing.T) {
	diff := DiffCategories([]string{"gh1"}, []string{})
	assert.Empty(t, diff.Shared)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, []string{"gh1"}, diff.GhOnly)
	assert.Len(t, diff.Issues, 1)
	assert.Contains(t, diff.Issues[0].Message, "exists in data/gh/ but not in dotfiles")
}

func TestDiffCategories_Mixed(t *testing.T) {
	diff := DiffCategories([]string{"shared", "gh-only"}, []string{"shared", "df-only"})
	assert.Equal(t, []string{"shared"}, diff.Shared)
	assert.Equal(t, []string{"df-only"}, diff.DfOnly)
	assert.Equal(t, []string{"gh-only"}, diff.GhOnly)
	assert.Len(t, diff.Issues, 2)
}

func TestDiffCategories_Empty(t *testing.T) {
	diff := DiffCategories(nil, nil)
	assert.Empty(t, diff.Shared)
	assert.Empty(t, diff.DfOnly)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.Issues)
}

func TestDiffCategories_Sorted(t *testing.T) {
	diff := DiffCategories([]string{"z", "a"}, []string{"z", "m"})
	assert.Equal(t, []string{"z"}, diff.Shared)
	assert.Equal(t, []string{"m"}, diff.DfOnly)
	assert.Equal(t, []string{"a"}, diff.GhOnly)
}

// --- classify ---

func TestClassify_GhOnly(t *testing.T) {
	state := map[string]*pkgPresence{
		"curl": {GHCats: []string{"utils"}},
	}
	diff := classify(state, nil)
	assert.Equal(t, []string{"utils"}, diff.GhOnly["curl"])
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Len(t, diff.Issues, 1)
}

func TestClassify_DfOnly(t *testing.T) {
	state := map[string]*pkgPresence{
		"vim": {DFCats: []string{"editor"}},
	}
	diff := classify(state, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Equal(t, []string{"editor"}, diff.DfOnly["vim"])
	assert.Equal(t, 0, diff.Shared)
	assert.Len(t, diff.Issues, 1)
}

func TestClassify_SharedSameCategory(t *testing.T) {
	state := map[string]*pkgPresence{
		"git": {GHCats: []string{"utils"}, DFCats: []string{"utils"}},
	}
	diff := classify(state, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 1, diff.Shared)
	assert.Empty(t, diff.Issues)
}

func TestClassify_CrossCategory(t *testing.T) {
	state := map[string]*pkgPresence{
		"git": {GHCats: []string{"utils"}, DFCats: []string{"vcs"}},
	}
	diff := classify(state, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Equal(t, CrossPkg{GHCats: []string{"utils"}, DFCats: []string{"vcs"}}, diff.CrossCategory["git"])
	assert.Len(t, diff.Issues, 1)
	assert.Contains(t, diff.Issues[0].Message, "cross-category")
}

func TestClassify_FalsePkg_SharedContradiction(t *testing.T) {
	state := map[string]*pkgPresence{
		"procs": {GHCats: []string{"kernel"}, DFCats: []string{"core"}},
	}
	falsePkgs := map[string]bool{"procs": true}
	diff := classify(state, falsePkgs)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Equal(t, []string{"core"}, diff.FalsePkgConflict["procs"])
	assert.Len(t, diff.Issues, 1)
	assert.Contains(t, diff.Issues[0].Message, "false-pkg-conflict")
	assert.Contains(t, diff.Issues[0].Message, "isDotfiles: false")
}

func TestClassify_FalsePkg_HasDfOnlyContradiction(t *testing.T) {
	// A package that exists in GH (as isDotfiles: false) AND in dotfiles
	// but the category is df-only (dotfiles only, not in GH map).
	// Actually this case can't happen by construction: if it's in ghMap
	// it would have GHCats. So the df-only branch is unaffected.
	// This test verifies the shared-with-overlap case.
	state := map[string]*pkgPresence{
		"tenv": {GHCats: []string{"devops"}, DFCats: []string{"devops"}},
	}
	falsePkgs := map[string]bool{"tenv": true}
	diff := classify(state, falsePkgs)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Equal(t, []string{"devops"}, diff.FalsePkgConflict["tenv"])
	assert.Len(t, diff.Issues, 1)
	assert.Contains(t, diff.Issues[0].Message, "false-pkg-conflict: tenv")
}

func TestDiffNix_FalsePkg_InDf(t *testing.T) {
	gh := map[string][]string{"procs": {"kernel"}}
	df := map[string][]string{"procs": {"core"}}
	falsePkgs := map[string]bool{"procs": true}
	diff := DiffNix(gh, df, falsePkgs, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Equal(t, []string{"core"}, diff.FalsePkgConflict["procs"])
	assert.Len(t, diff.Issues, 1)
	assert.Contains(t, diff.Issues[0].Message, "false-pkg-conflict")
}

func TestClassify_Empty(t *testing.T) {
	diff := classify(map[string]*pkgPresence{}, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
	assert.Empty(t, diff.CrossCategory)
	assert.Empty(t, diff.Issues)
}

// --- DiffNix ---

func TestDiffNix_Shared(t *testing.T) {
	gh := map[string][]string{"curl": {"utils"}}
	df := map[string][]string{"curl": {"utils"}}
	diff := DiffNix(gh, df, nil, nil)
	assert.Equal(t, 1, diff.Shared)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
}

func TestDiffNix_GhOnly(t *testing.T) {
	gh := map[string][]string{"gcc": {"build"}}
	df := map[string][]string{}
	diff := DiffNix(gh, df, nil, nil)
	assert.Equal(t, []string{"build"}, diff.GhOnly["gcc"])
	assert.Len(t, diff.Issues, 1)
}

func TestDiffNix_DfOnly(t *testing.T) {
	gh := map[string][]string{}
	df := map[string][]string{"vim": {"editor"}}
	diff := DiffNix(gh, df, nil, nil)
	assert.Equal(t, []string{"editor"}, diff.DfOnly["vim"])
	assert.Len(t, diff.Issues, 1)
}

func TestDiffNix_SelfBuiltFiltered(t *testing.T) {
	gh := map[string][]string{"my-pkg": {"custom"}}
	df := map[string][]string{"my-pkg": {"custom"}}
	selfBuilt := map[string]bool{"my-pkg": true}
	diff := DiffNix(gh, df, nil, selfBuilt)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
}

func TestDiffNix_PrefixFiltered(t *testing.T) {
	gh := map[string][]string{"gnomeExtensions.caffeine": {"desktop"}}
	df := map[string][]string{"gnomeExtensions.caffeine": {"desktop"}}
	diff := DiffNix(gh, df, nil, nil)
	assert.Empty(t, diff.GhOnly)
	assert.Empty(t, diff.DfOnly)
	assert.Equal(t, 0, diff.Shared)
}

func TestDiffNix_FalsePkg(t *testing.T) {
	gh := map[string][]string{"gcc": {"build"}}
	df := map[string][]string{}
	falsePkgs := map[string]bool{"gcc": true}
	diff := DiffNix(gh, df, falsePkgs, nil)
	assert.Equal(t, 1, diff.Shared)
	assert.Empty(t, diff.GhOnly)
}

// --- MergeResult ---

func TestMergeResult_CombinesIssues(t *testing.T) {
	cat := &CatDiff{
		Issues: []checkutil.Issue{{Message: "cat issue", Severity: checkutil.SeverityError}},
	}
	nix := NixDiff{
		Issues: []checkutil.Issue{{Message: "nix issue", Severity: checkutil.SeverityError}},
	}
	result := MergeResult(cat, nix)
	assert.Len(t, result.Issues, 2)
}

func TestMergeResult_Sorted(t *testing.T) {
	cat := &CatDiff{
		Issues: []checkutil.Issue{{Message: "z issue", Severity: checkutil.SeverityError}},
	}
	nix := NixDiff{
		Issues: []checkutil.Issue{{Message: "a issue", Severity: checkutil.SeverityError}},
	}
	result := MergeResult(cat, nix)
	assert.Equal(t, "a issue", result.Issues[0].Message)
	assert.Equal(t, "z issue", result.Issues[1].Message)
}

func TestMergeResult_Summary(t *testing.T) {
	cat := &CatDiff{
		Shared: []string{"a"},
		DfOnly: []string{"b"},
		GhOnly: []string{"c"},
	}
	nix := NixDiff{
		GhOnly:           map[string][]string{"x": {"cat"}},
		DfOnly:           map[string][]string{"y": {"cat"}},
		CrossCategory:    map[string]CrossPkg{"z": {}},
		FalsePkgConflict: map[string][]string{"bad": {"cat"}},
		Shared:           5,
	}
	result := MergeResult(cat, nix)
	assert.Equal(t, 1, result.Summary["shared"])
	assert.Equal(t, 1, result.Summary["dfOnly"])
	assert.Equal(t, 1, result.Summary["ghOnly"])
	assert.Equal(t, 1, result.Summary["nixGhOnly"])
	assert.Equal(t, 1, result.Summary["nixDfOnly"])
	assert.Equal(t, 1, result.Summary["nixCrossCat"])
	assert.Equal(t, 1, result.Summary["nixFalsePkgConflict"])
	assert.Equal(t, 5, result.Summary["nixShared"])
}

func TestMergeResult_Empty(t *testing.T) {
	result := MergeResult(&CatDiff{}, NixDiff{})
	assert.Empty(t, result.Issues)
	assert.NotNil(t, result.Summary)
}

// --- FilterGhOnlyCategories ---

func TestFilterGhOnlyCategories_ExcludesNoDotfiles(t *testing.T) {
	ghDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "ghonly"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ghDir, "ghonly", "type.yml"),
		[]byte("- isDotfiles: false\n"),
		0o600,
	))

	diff := &CatDiff{
		GhOnly: []string{"ghonly"},
		Issues: []checkutil.Issue{
			{File: "category:ghonly", Severity: checkutil.SeverityError, Message: "gh-only issue"},
		},
	}
	filtered, err := FilterGhOnlyCategories(diff, ghDir)
	require.NoError(t, err)
	assert.Empty(t, filtered.GhOnly)
	assert.Empty(t, filtered.Issues)
}

func TestFilterGhOnlyCategories_KeepsNormalCategory(t *testing.T) {
	ghDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "normal"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ghDir, "normal", "type.yml"),
		[]byte("- isDotfiles: true\n"),
		0o600,
	))

	diff := &CatDiff{
		GhOnly: []string{"normal"},
		Issues: []checkutil.Issue{
			{File: "category:normal", Severity: checkutil.SeverityError, Message: "gh-only issue"},
		},
	}
	filtered, err := FilterGhOnlyCategories(diff, ghDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"normal"}, filtered.GhOnly)
	assert.Len(t, filtered.Issues, 1)
}

func TestFilterGhOnlyCategories_PreservesNonGhOnlyIssues(t *testing.T) {
	ghDir := t.TempDir()

	diff := &CatDiff{
		DfOnly: []string{"dfcat"},
		GhOnly: []string{},
		Issues: []checkutil.Issue{
			{File: "category:dfcat", Severity: checkutil.SeverityError, Message: "df-only issue"},
		},
	}
	filtered, err := FilterGhOnlyCategories(diff, ghDir)
	require.NoError(t, err)
	assert.Empty(t, filtered.GhOnly)
	assert.Len(t, filtered.Issues, 1)
	assert.Contains(t, filtered.Issues[0].Message, "df-only")
}

func TestFilterGhOnlyCategories_MixedExclusion(t *testing.T) {
	ghDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "excluded"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ghDir, "excluded", "type.yml"),
		[]byte("- isDotfiles: false\n"),
		0o600,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "kept"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ghDir, "kept", "type.yml"),
		[]byte("- isDotfiles: true\n"),
		0o600,
	))

	diff := &CatDiff{
		GhOnly: []string{"excluded", "kept"},
		Issues: []checkutil.Issue{
			{File: "category:excluded", Severity: checkutil.SeverityError, Message: "excluded issue"},
			{File: "category:kept", Severity: checkutil.SeverityError, Message: "kept issue"},
		},
	}
	filtered, err := FilterGhOnlyCategories(diff, ghDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"kept"}, filtered.GhOnly)
	assert.Len(t, filtered.Issues, 1)
	assert.Contains(t, filtered.Issues[0].Message, "kept")
}

// --- FormatCompact ---

func TestFormatCompact(t *testing.T) {
	r := &DiffResult{
		Category: CatDiff{Shared: []string{"a"}, DfOnly: []string{"b"}, GhOnly: []string{"c"}},
		Nix:      NixDiff{GhOnly: map[string][]string{"x": {}}, DfOnly: map[string][]string{"y": {}}},
	}
	s := FormatCompact(r)
	assert.Contains(t, s, "categories shared=1 df-only=1 gh-only=1")
	assert.Contains(t, s, "nix gh-only=1 df-only=1")
}

// --- NixDiff.Summary ---

func TestNixDiffSummary(t *testing.T) {
	d := &NixDiff{
		GhOnly:        map[string][]string{"a": {}},
		DfOnly:        map[string][]string{"b": {}, "c": {}},
		CrossCategory: map[string]CrossPkg{"d": {}},
		Shared:        5,
	}
	s := d.Summary()
	assert.Equal(t, 1, s["ghOnly"])
	assert.Equal(t, 2, s["dfOnly"])
	assert.Equal(t, 1, s["crossCategory"])
	assert.Equal(t, 5, s["shared"])
}
