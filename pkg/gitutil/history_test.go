package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsSubstantive(t *testing.T) {
	require.True(t, isSubstantive(40, 0, 40, 2))
	require.True(t, isSubstantive(0, 2, 40, 2))
	require.False(t, isSubstantive(39, 1, 40, 2))
}

func TestContentDeltaWhitespaceOnly(t *testing.T) {
	c, l, diff := contentDelta("hello\n", "hello \n", 1000)
	require.Equal(t, 0, c)
	require.Equal(t, 0, l)
	require.Empty(t, diff)
}

func TestContentDeltaAppend(t *testing.T) {
	old := "line1\n"
	newC := "line1\n[interesting long paper about agents routing](https://example.com/very/long/path/to/paper)\n"
	c, l, diff := contentDelta(old, newC, 1000)
	require.GreaterOrEqual(t, c, 40)
	require.GreaterOrEqual(t, l, 1)
	require.Contains(t, diff, "interesting long paper")
}

func TestCollectLogEditsIntegration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	// Pin commit times so "init" is outside the since window while later commits are inside.
	// Without this, a brand-new temp repo puts init in the same 24h window as the edit.
	commitAt := func(when time.Time, msg string) {
		ts := when.UTC().Format(time.RFC3339)
		cmd := exec.Command("git", "commit", "-m", msg)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE="+ts,
			"GIT_COMMITTER_DATE="+ts,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")

	now := time.Now().UTC()
	// Initial commit — outside since window
	require.NoError(t, os.MkdirAll(filepath.Join(root, "wiki/a/b/c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "wiki/a/b/c/log.md"), []byte("# log\n"), 0o644))
	run("add", ".")
	commitAt(now.Add(-72*time.Hour), "init")

	// Substantive edit — inside window
	require.NoError(t, os.WriteFile(filepath.Join(root, "wiki/a/b/c/log.md"),
		[]byte("# log\n\n[interesting paper about agents](https://example.com/paper)\n"), 0o644))
	run("add", ".")
	commitAt(now.Add(-2*time.Hour), "edit log")

	// Bulk commit: create many logs — inside window, must be dropped as a whole
	for i := 0; i < 12; i++ {
		dir := filepath.Join(root, "wiki/x/y", "t"+strconv.Itoa(i))
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "log.md"),
			[]byte("bulk noise content that is long enough 0123456789\n"), 0o644))
	}
	run("add", ".")
	commitAt(now.Add(-1*time.Hour), "bulk reorg")

	// Pure rename without content change — must not produce an edit
	require.NoError(t, os.MkdirAll(filepath.Join(root, "wiki/a/b/d"), 0o755))
	run("mv", "wiki/a/b/c/log.md", "wiki/a/b/d/log.md")
	commitAt(now.Add(-30*time.Minute), "rename log")

	since := now.Add(-24 * time.Hour)
	edits, err := CollectLogEdits(root, &CollectLogEditOptions{
		Since:            since,
		BulkLogThreshold: 10,
		MinDeltaChars:    40,
		MinDeltaLines:    2,
		PathPrefix:       "wiki",
	})
	require.NoError(t, err)

	// Exactly one substantive content edit; bulk commit contributes zero;
	// pure rename (no content delta) contributes zero; init is outside window.
	require.Len(t, edits, 1, "expected exactly one substantive log edit (bulk+rename+old init excluded)")
	// After exact rename, go-git may attribute the blob to the new path.
	require.Contains(t, []string{
		"wiki/a/b/c/log.md",
		"wiki/a/b/d/log.md",
	}, edits[0].Path, "edit path should be pre- or post-rename log.md")
	require.GreaterOrEqual(t, edits[0].DeltaChars, 40)
	require.Contains(t, edits[0].Diff, "interesting paper about agents")
	for _, e := range edits {
		require.NotContains(t, e.Path, "wiki/x/y/", "bulk-created log paths must not appear")
	}
}

func TestCollectLogEditsUntilUpperBound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	commitAt := func(when time.Time, msg string) {
		ts := when.UTC().Format(time.RFC3339)
		cmd := exec.Command("git", "commit", "-m", msg)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t",
			"GIT_AUTHOR_DATE="+ts,
			"GIT_COMMITTER_DATE="+ts,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t",
			"GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")

	// Fixed calendar: June in-window edit, July out-of-window edit.
	june := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "wiki/a/b/c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "wiki/a/b/c/log.md"), []byte("# log\n"), 0o644))
	run("add", ".")
	commitAt(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "init")

	require.NoError(t, os.WriteFile(filepath.Join(root, "wiki/a/b/c/log.md"),
		[]byte("# log\n\n[june paper about agents routing](https://example.com/june)\n"), 0o644))
	run("add", ".")
	commitAt(june, "june edit")

	require.NoError(t, os.WriteFile(filepath.Join(root, "wiki/a/b/c/log.md"),
		[]byte("# log\n\n[june paper about agents routing](https://example.com/june)\n\n[july paper about agents routing](https://example.com/july)\n"), 0o644))
	run("add", ".")
	commitAt(july, "july edit")

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	edits, err := CollectLogEdits(root, &CollectLogEditOptions{
		Since:            since,
		Until:            until,
		BulkLogThreshold: 10,
		MinDeltaChars:    40,
		MinDeltaLines:    2,
		PathPrefix:       "wiki",
	})
	require.NoError(t, err)
	require.Len(t, edits, 1, "only June edit is in [June 1, July 1)")
	require.Contains(t, edits[0].Diff, "june paper")
	require.NotContains(t, edits[0].Diff, "july paper")
}

func TestTopicDirFromLogPath(t *testing.T) {
	require.Equal(t, "wiki/a/b/c", TopicDirFromLogPath("wiki/a/b/c/log.md"))
}
