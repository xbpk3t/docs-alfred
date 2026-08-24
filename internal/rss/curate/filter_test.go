package curate

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
)

var testNow = time.Now()

func TestValidFeedURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com/feed.xml", true},
		{"http://blog/feed", true},
		{"ftp://example.com/a", false},
		{"//example.com/no-scheme", false},
		{"https://localhost/feed", false},
		{"not a url", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, validFeedURL(c.in), "validFeedURL(%q)", c.in)
	}
}

func TestFinalizeCandidateAliasesAndRefusals(t *testing.T) {
	known := map[string]string{"bz视频": "https://rss.example/bz.xml"}

	// Known aggregate alias -> reuse feed directly.
	alias := Candidate{Name: "bz视频", Feed: ""}
	finalizeCandidate(&alias, known)
	assert.Equal(t, "https://rss.example/bz.xml", alias.Feed)
	assert.Empty(t, alias.Fatal)

	// Missing name -> refused.
	noName := Candidate{Feed: "https://x.example/f.xml"}
	finalizeCandidate(&noName, known)
	assert.Equal(t, noRSS, noName.Fatal)

	// Hallucinated feed (not a URL) -> refused no_rss.
	bad := Candidate{Name: "新源"}
	finalizeCandidate(&bad, known)
	assert.Equal(t, noRSS, bad.Fatal)

	// Real URL passes.
	good := Candidate{Name: "DeepSeek blog", Feed: "https://api-docs.deepseek.com/feed.xml"}
	finalizeCandidate(&good, known)
	assert.Empty(t, good.Fatal)
}

func TestFreqFromFeedGate(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{Posts90DMin: 10, LastMaxDays: 60}

	// 12 posts within the window -> pass.
	busy := &gofeed.Feed{}
	for i := 0; i < 12; i++ {
		ts := now.Add(-24 * time.Hour * time.Duration(i)) // daily, recent
		busy.Items = append(busy.Items, &gofeed.Item{Title: "t", PublishedParsed: &ts})
	}
	r := freqFromFeed(&Candidate{Name: "active"}, busy, cfg, now)
	assert.Equal(t, statusPass, r.Status)
	assert.Contains(t, r.Reason, "posts90d")

	// Too stale: 12 posts but all >60 days ago -> freq_low.
	stale := &gofeed.Feed{}
	for i := 0; i < 12; i++ {
		ts := now.Add(-72 * 24 * time.Hour)
		stale.Items = append(stale.Items, &gofeed.Item{Title: "t", PublishedParsed: &ts})
	}
	r = freqFromFeed(&Candidate{Name: "stale"}, stale, cfg, now)
	assert.Equal(t, statusFreqLow, r.Status)

	// Too few posts (fresh but sparse) -> freq_low.
	sparse := &gofeed.Feed{}
	for i := 0; i < 3; i++ {
		ts := now.Add(-24 * time.Hour)
		sparse.Items = append(sparse.Items, &gofeed.Item{Title: "t", PublishedParsed: &ts})
	}
	r = freqFromFeed(&Candidate{Name: "sparse"}, sparse, cfg, now)
	assert.Equal(t, statusFreqLow, r.Status)
}

func TestGroupKeepsByTypeShapesRSSYAML(t *testing.T) {
	keeps := []keepEntry{
		{Type: "blog", Feed: "https://a.example/feed.xml", URL: "https://a.example/", Des: "A"},
		{Type: "blog", Feed: "https://b.example/feed.xml"},
		{Type: "videos", Feed: "https://y.example/ch.xml", IsMedia: true},
	}
	detail := groupKeepsByType(keeps)
	require.Len(t, detail, 2)
	by := map[string]*rss.FeedsDetail{}
	for i := range detail {
		by[detail[i].Type] = &detail[i]
	}
	assert.Len(t, by["blog"].Feeds, 2)
	assert.True(t, by["videos"].Feeds[0].IsMedia)
	assert.Equal(t, "A", by["blog"].Feeds[0].Des)

	y, err := renderKeepYAML(detail, testNow)
	require.NoError(t, err)
	assert.Contains(t, string(y), "# --- bestblogs import ")
}
