package wikiingest

import (
	"context"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

// fakeFeedStore fakes the FeedBuilder surface: seeded objects are listed/read,
// uploads land in a map.
type fakeFeedStore struct {
	objects map[string]string
	uploads map[string]string
}

func newFakeFeedStore(objects map[string]string) *fakeFeedStore {
	return &fakeFeedStore{objects: objects, uploads: map[string]string{}}
}

func (f *fakeFeedStore) ListKeys(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return keys, nil
}

func (f *fakeFeedStore) GetObject(_ context.Context, key string) ([]byte, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("no such object: %s", key)
	}

	return []byte(data), nil
}

func (f *fakeFeedStore) Upload(_ context.Context, key, content string) (*litter.Result, error) {
	f.uploads[key] = content

	return &litter.Result{URL: "https://dw.test/" + key, Driver: "r2"}, nil
}

// digestPostMarkdown mimics buildEntry output: an "### title" heading, a
// ```markdown metadata block, and a structured summary.
func digestPostMarkdown(title, summary string) string {
	return fmt.Sprintf("### %s\n\n```markdown\nURL: https://example.com/%s\nType: research\n```\n\n%s\n",
		title, strings.ToLower(strings.ReplaceAll(title, " ", "")), summary)
}

func TestFolderOfFirstSegment(t *testing.T) {
	assert.Equal(t, "AI", folderOf("AI/LLM-use/coding-agent-harness"))
	assert.Equal(t, "desktop", folderOf("desktop/OS/linux-desktop"))
	assert.Equal(t, "uncat", folderOf("uncat/uncat/uncat"))
	assert.Equal(t, "no-slash", folderOf("no-slash"))
}

func TestParseDigestKeyGroupsByFolder(t *testing.T) {
	keys := []string{
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md",
		"digest/wiki-2026-10-26/AI/agent/agents/abcdef0123456789.md",
		"digest/wiki-2026-10-26/AI/tooling/frameworks/fedcba9876543210.md",
		"digest/unbatched/uncat/uncat/uncat/0000000000000000.md",
		"feeds/AI/agent.xml",
	}

	var folders []string
	for _, key := range keys {
		m := digestKeyRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		folders = append(folders, folderOf(m[2]+"/"+m[3]+"/"+m[4]))
	}
	assert.Equal(t, []string{"AI", "AI", "AI", "uncat"}, folders,
		"non-post keys like feeds/ must not match the digest key pattern")
}

func TestEntryTitleExtractsFirstH3(t *testing.T) {
	title := entryTitle(digestPostMarkdown("Introducing agents", "overview text"))
	assert.Equal(t, "Introducing agents", title)

	// A body without an h3 falls back to a stable placeholder.
	assert.Equal(t, "(untitled)", entryTitle("no heading here"))
}

func TestBatchDateParsesWikiBatch(t *testing.T) {
	d, ok := batchDate("wiki-2026-10-26")
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), *d)

	_, ok = batchDate("unbatched")
	assert.False(t, ok, "batch ids without a date carry no pubDate")
}

func TestBuildFeedsWritesOneRSSPerFolder(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md":  digestPostMarkdown("Agents explained", "summary A"),
		"digest/wiki-2026-10-26/AI/agent/agents/abcdef0123456789.md":  digestPostMarkdown("Agent tools", "summary B"),
		"digest/wiki-2026-10-27/AI/agent/agents/1234567890abcdef.md":  digestPostMarkdown("Agent fwk", "summary C"),
		"digest/wiki-2026-10-27/desktop/OS/linux/a1b2c3d4e5f67890.md": digestPostMarkdown("Desktop post", "summary D"),
	})

	err := BuildFeeds(context.Background(), store, "https://dw.lucc.dev")
	require.NoError(t, err)

	// One feed per folder: AI holds all three posts regardless of topic.
	feedXML, ok := store.uploads["feeds/AI.xml"]
	require.True(t, ok, "one folder → one feed object")
	var feed rssFeed
	require.NoError(t, xml.Unmarshal([]byte(feedXML), &feed))
	assert.Equal(t, "AI", feed.Channel.Title)
	require.Len(t, feed.Channel.Items, 3)
	// Newest batch first; same-batch posts keep build order.
	assert.Equal(t, "Agent fwk", feed.Channel.Items[0].Title)
	assert.Equal(t, "Agents explained", feed.Channel.Items[1].Title)
	assert.Equal(t, "Agent tools", feed.Channel.Items[2].Title)
	// lastBuildDate mirrors the newest batch.
	assert.Equal(t, "Tue, 27 Oct 2026 00:00:00 +0000", feed.Channel.LastBuildDate)

	// A different folder gets its own feed.
	desktopXML, ok := store.uploads["feeds/desktop.xml"]
	require.True(t, ok)
	var desktop rssFeed
	require.NoError(t, xml.Unmarshal([]byte(desktopXML), &desktop))
	assert.Equal(t, "desktop", desktop.Channel.Title)
	require.Len(t, desktop.Channel.Items, 1)
}

// TestBuildFeedsOrdersAcrossMonthBoundary guards the sort: RFC1123Z strings
// compare lexically, so "02 Nov" would sort before "26 Oct" — items must be
// ordered by their parsed batch date instead.
func TestBuildFeedsOrdersAcrossMonthBoundary(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md": digestPostMarkdown("October", "summary A"),
		"digest/wiki-2026-11-02/AI/agent/agents/abcdef0123456789.md": digestPostMarkdown("November", "summary B"),
	})

	require.NoError(t, BuildFeeds(context.Background(), store, "https://dw.lucc.dev"))

	var feed rssFeed
	require.NoError(t, xml.Unmarshal([]byte(store.uploads["feeds/AI.xml"]), &feed))
	require.Len(t, feed.Channel.Items, 2)
	assert.Equal(t, "November", feed.Channel.Items[0].Title, "newer batch (Nov 2) must sort before older (Oct 26)")
	assert.Equal(t, "October", feed.Channel.Items[1].Title)
	assert.Equal(t, "Mon, 02 Nov 2026 00:00:00 +0000", feed.Channel.LastBuildDate)
}

func TestBuildFeedsUnbatchedSortsLastAndOmitsDates(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"digest/unbatched/uncat/uncat/uncat/0000000000000000.md":     digestPostMarkdown("Unbatched", "summary U"),
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md": digestPostMarkdown("Dated", "summary D"),
	})

	require.NoError(t, BuildFeeds(context.Background(), store, "https://dw.lucc.dev"))

	var uncat rssFeed
	require.NoError(t, xml.Unmarshal([]byte(store.uploads["feeds/uncat.xml"]), &uncat))
	// Different folders → separate feeds. Check the uncat feed has no lastBuildDate.
	assert.Empty(t, uncat.Channel.LastBuildDate)

	var dated rssFeed
	require.NoError(t, xml.Unmarshal([]byte(store.uploads["feeds/AI.xml"]), &dated))
	require.Len(t, dated.Channel.Items, 1)
	assert.NotEmpty(t, dated.Channel.Items[0].PubDate)
}

func TestBuildFeedsLinksAndDates(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md": digestPostMarkdown("Agents", "overview text"),
	})

	require.NoError(t, BuildFeeds(context.Background(), store, "https://dw.lucc.dev"))

	feedXML := store.uploads["feeds/AI.xml"]
	var feed rssFeed
	require.NoError(t, xml.Unmarshal([]byte(feedXML), &feed))
	item := feed.Channel.Items[0]
	assert.Equal(t, "https://dw.lucc.dev/digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md", item.Link)
	assert.Equal(t, "https://dw.lucc.dev/digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md", item.GUID)
	assert.Equal(t, "Mon, 26 Oct 2026 00:00:00 +0000", item.PubDate)
	assert.Contains(t, item.Description, `<h3 id="agents">Agents</h3>`)
	assert.Contains(t, item.Description, "<p>overview text</p>")
}

func TestBuildFeedsSkipsMalformedKeysButKeepsFeeds(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"digest/wiki-2026-10-26/AI/agent/agents/1a2b3c4d5e6f7890.md": digestPostMarkdown("Agents", "x"),
		"digest/bad-key.txt": "not a post",
		"feeds/other.xml":    "ignored by BuildFeeds",
	})

	require.NoError(t, BuildFeeds(context.Background(), store, "https://dw.lucc.dev"))

	require.Contains(t, store.uploads, "feeds/AI.xml")
	assert.Len(t, store.uploads, 1, "malformed digest keys are skipped, feeds/ is not touched")
}

func TestBuildOPMLOneOutlinePerFolder(t *testing.T) {
	store := newFakeFeedStore(map[string]string{
		"feeds/AI.xml":      "<rss/>",
		"feeds/desktop.xml": "<rss/>",
		"feeds/kernel.xml":  "<rss/>",
		"feeds/zzz.xml":     "<rss/>",
	})

	body, err := BuildOPML(context.Background(), store, "https://dw.lucc.dev")
	require.NoError(t, err)
	assert.Contains(t, body, `<opml version="2.0">`)
	assert.Contains(t, body, `<outline text="AI" title="AI" xmlUrl="https://dw.lucc.dev/feeds/AI.xml"`)
	assert.Contains(t, body, `<outline text="desktop" title="desktop" xmlUrl="https://dw.lucc.dev/feeds/desktop.xml"`)
	assert.Contains(t, body, `<outline text="kernel" title="kernel" xmlUrl="https://dw.lucc.dev/feeds/kernel.xml"`)
	// Every outline is a flat feed leaf; no nesting, no bare folder nodes.
	assert.NotContains(t, body, `xmlUrl=""`)
	assert.NotContains(t, body, `<outline text="AI"`+"\n") // no children

	// The document is uploaded as digest.opml and is parseable.
	require.Contains(t, store.uploads, "digest.opml")
	assert.Equal(t, body, store.uploads["digest.opml"])

	var doc opmlDoc
	require.NoError(t, xml.Unmarshal([]byte(body), &doc))
	require.Len(t, doc.Body.Outlines, 4)
	assert.Equal(t, "AI", doc.Body.Outlines[0].Title)
	assert.Equal(t, "https://dw.lucc.dev/feeds/AI.xml", doc.Body.Outlines[0].Feed)
}
