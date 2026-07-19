package blog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
)

func init() {
	carboninit.Setup()
}

func TestBlogInWindow_DatePreferredOverFreshMtime(t *testing.T) {
	// Old authoring date + "fresh" mtime (reorg) must NOT cool.
	since := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	b := BlogMeta{
		Path:    "/tmp/2026-06-01-old-piece.md",
		Date:    "2026-06-01",
		ModTime: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC), // within window
	}
	require.False(t, blogInWindow(b, since), "frontmatter date outside window wins over reorg mtime")
}

func TestBlogInWindow_RecentFrontmatterDate(t *testing.T) {
	since := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	b := BlogMeta{
		Path:    "/tmp/note.md",
		Date:    "2026-07-15",
		ModTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), // stale mtime ignored
	}
	require.True(t, blogInWindow(b, since))
}

func TestBlogInWindow_FilenameDate(t *testing.T) {
	since := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	b := BlogMeta{
		Path:    "/topic/2026-07-16-fresh-blog.md",
		Date:    "",
		ModTime: time.Time{},
	}
	require.True(t, blogInWindow(b, since))

	old := BlogMeta{
		Path: "/topic/2026-01-01-old.md",
		Date: "",
	}
	require.False(t, blogInWindow(old, since))
}

func TestBlogInWindow_MtimeOnlyWhenNoDate(t *testing.T) {
	since := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	// No date, no filename prefix → mtime fallback
	fresh := BlogMeta{
		Path:    "/topic/untitled.md",
		Date:    "",
		ModTime: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
	}
	require.True(t, blogInWindow(fresh, since))

	stale := BlogMeta{
		Path:    "/topic/untitled.md",
		Date:    "",
		ModTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	require.False(t, blogInWindow(stale, since))
}

func TestTopicHasNewBlogInWindow_Integration(t *testing.T) {
	dir := t.TempDir()
	// Recent frontmatter date
	require.NoError(t, os.WriteFile(filepath.Join(dir, "piece.md"), []byte(`---
title: Recent
date: 2026-07-18
type: blog
---
body
`), 0o600))
	// Old frontmatter but we will not trust mtime (file is new on disk)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "2026-01-02-old.md"), []byte(`---
title: Old
date: 2026-01-02
type: blog
---
body
`), 0o600))

	since := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	ok, recent, err := TopicHasNewBlogInWindow(dir, since)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, recent, 1)
	require.Equal(t, "Recent", recent[0].Title)
}

func TestListTopicBlogs_SkipsLogAndSummary(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "log.md"), []byte("---\ntype: blog\ntitle: x\n---\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "summary.md"), []byte("---\ntype: blog\ntitle: y\n---\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.md"), []byte("---\ntype: blog\ntitle: z\ndate: 2026-07-01\n---\n"), 0o600))

	blogs, err := ListTopicBlogs(dir)
	require.NoError(t, err)
	require.Len(t, blogs, 1)
	require.Equal(t, "z", blogs[0].Title)
}

func TestBlogTitles(t *testing.T) {
	out := BlogTitles([]BlogMeta{
		{Date: "2026-07-01", Title: "A"},
		{Title: "B"},
	})
	require.Equal(t, []string{"2026-07-01 — A", "B"}, out)
}
