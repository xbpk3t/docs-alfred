package compact

import (
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/prompt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/gitutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

func init() {
	carboninit.Setup()
	validator.Setup()
}

func TestAggregateHotTopicsScoring(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	edits := []gitutil.LogEdit{
		{Path: "wiki/AI/LLM/LLM/log.md", CommitHash: "a", When: now, DeltaChars: 100, Diff: "+ a"},
		{Path: "wiki/AI/LLM/LLM/log.md", CommitHash: "b", When: now.Add(-24 * time.Hour), DeltaChars: 50, Diff: "+ b"},
		{Path: "wiki/infra/infra/proxy/log.md", CommitHash: "c", When: now, DeltaChars: 40, Diff: "+ c"},
	}
	hots := AggregateHotTopics(edits, "wiki")
	require.Len(t, hots, 2)
	require.Equal(t, "AI/LLM/LLM", hots[0].TopicPath)
	require.Equal(t, 2, hots[0].EditDays)
	require.Equal(t, 2, hots[0].EditCommits)
	require.Greater(t, hots[0].Score, hots[1].Score)
}

func TestTopNHot(t *testing.T) {
	in := []HotTopic{{TopicPath: "a"}, {TopicPath: "b"}, {TopicPath: "c"}}
	require.Len(t, TopNHot(in, 2), 2)
	require.Len(t, TopNHot(in, 10), 3)
}

func TestSelectNoticesOnlyYes(t *testing.T) {
	in := []CompactRecommend{
		{Recommend: "no", Topic: HotTopic{TopicPath: "a"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "b"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "c"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "d"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "e"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "f"}},
	}
	out := SelectNotices(in, 5)
	require.Len(t, out, 5)
	require.Equal(t, "b", out[0].Topic.TopicPath)
	require.Equal(t, "f", out[4].Topic.TopicPath)
}

func TestSelectNoticesSkipsCooling(t *testing.T) {
	in := []CompactRecommend{
		{Recommend: "yes", SkippedCooling: true, Topic: HotTopic{TopicPath: "a"}},
		{Recommend: "yes", Topic: HotTopic{TopicPath: "b"}},
	}
	out := SelectNotices(in, 5)
	require.Len(t, out, 1)
	require.Equal(t, "b", out[0].Topic.TopicPath)
}

func TestParseCompactJSONAcceptsBareObject(t *testing.T) {
	raw := `{"recommend":"yes","why":["x"],"suggested_angle":"slice"}`
	r, err := parseCompactJSON(raw)
	require.NoError(t, err)
	require.Equal(t, "yes", r.Recommend)
	require.Equal(t, "slice", r.SuggestedAngle)
}

func TestParseCompactJSONRejectsCodeFence(t *testing.T) {
	raw := "```json\n{\"recommend\":\"yes\",\"why\":[\"x\"],\"suggested_angle\":\"slice\"}\n```"
	_, err := parseCompactJSON(raw)
	require.Error(t, err, "should reject code-fence-wrapped JSON")
}

func TestParseCompactJSONRejectsTrailingText(t *testing.T) {
	raw := `{"recommend":"yes","why":["x"],"suggested_angle":"slice"}` + "\nsome trailing text"
	_, err := parseCompactJSON(raw)
	require.Error(t, err, "should reject trailing text after JSON")
}

func TestParseCompactJSONValidatesRecommend(t *testing.T) {
	_, err := parseCompactJSON(`{"recommend":"yes","why":["x"],"suggested_angle":"a"}`)
	require.NoError(t, err)
	_, err = parseCompactJSON(`{"recommend":"no","why":[],"suggested_angle":""}`)
	require.NoError(t, err)
	_, err = parseCompactJSON(`{"recommend":"YES","why":[],"suggested_angle":""}`)
	require.NoError(t, err, "case-normalized before Struct")
	_, err = parseCompactJSON(`{"recommend":"maybe","why":[],"suggested_angle":""}`)
	require.Error(t, err)
	_, err = parseCompactJSON(`{"why":[],"suggested_angle":""}`)
	require.Error(t, err, "missing recommend")
}

func TestParseWindowLastMonth(t *testing.T) {
	// Asia/Shanghai after carboninit.Setup; pin wall clock mid-July.
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, loc)

	for _, token := range []string{"", "last-month", "prev-month", "LAST-MONTH"} {
		win, err := ParseWindow(token, now)
		require.NoError(t, err, token)
		require.Equal(t, "last-month", win.Label, token)
		// Start/end should be June 1 00:00 and July 1 00:00 Shanghai.
		require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc).Unix(), win.Start.Unix(), token)
		require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, loc).Unix(), win.End.Unix(), token)
	}
}

func TestParseWindowLastMonthOnFirst(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	// Monthly CI runs ~05:00 on the 1st → still previous calendar month.
	now := time.Date(2026, 7, 1, 5, 0, 0, 0, loc)
	win, err := ParseWindow("last-month", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc).Unix(), win.Start.Unix())
	require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, loc).Unix(), win.End.Unix())
}

func TestParseWindowLastMonthJanuary(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 1, 3, 5, 0, 0, 0, loc)
	win, err := ParseWindow("last-month", now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2025, 12, 1, 0, 0, 0, 0, loc).Unix(), win.Start.Unix())
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Unix(), win.End.Unix())
}

func TestParseWindowRolling(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	win, err := ParseWindow("7d", now)
	require.NoError(t, err)
	require.Equal(t, "7d", win.Label)
	require.Equal(t, now.Add(-7*24*time.Hour), win.Start)
	require.Equal(t, now, win.End)

	win, err = ParseWindow("30d", now)
	require.NoError(t, err)
	require.Equal(t, "30d", win.Label)
	require.Equal(t, now.Add(-30*24*time.Hour), win.Start)

	win, err = ParseWindow("168h", now)
	require.NoError(t, err)
	require.Equal(t, "7d", win.Label)
}

func TestParseSince(t *testing.T) {
	d, err := ParseSince("7d")
	require.NoError(t, err)
	require.Equal(t, 7*24*time.Hour, d)
	d, err = ParseSince("last-month")
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), d)
}

func TestNormalizeCompactOptsDefaults(t *testing.T) {
	opts := CompactOptions{}
	normalizeCompactOpts(&opts)
	require.Equal(t, 10, opts.TopHot)
	require.Equal(t, 5, opts.TopNotice)
	require.Equal(t, 10, opts.BulkLogThreshold)
	require.Equal(t, 40, opts.MinDeltaChars)
	require.Equal(t, 2, opts.MinDeltaLines)
}

func TestRenderCompactSubject(t *testing.T) {
	day := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	require.Contains(t, RenderCompactSubject(&CompactMailInput{Date: day, AISkipped: true}), "AI skipped")
	require.Contains(t, RenderCompactSubject(&CompactMailInput{Date: day, AISkipped: true, SkipAI: true}), "--skip-ai")
	require.Contains(t, RenderCompactSubject(&CompactMailInput{Date: day}), "none")
	require.Contains(t, RenderCompactSubject(&CompactMailInput{
		Date:    day,
		Notices: []CompactRecommend{{Recommend: "yes"}, {Recommend: "yes"}},
	}), "2 notices")
}

func TestRenderCompactHTMLWithNotices(t *testing.T) {
	in := CompactMailInput{
		Date:  time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC),
		Since: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Params: CompactParams{
			SinceDuration: "last-month",
			BulkThreshold: 10,
			MinDeltaChars: 40,
			MinDeltaLines: 2,
			TopHot:        10,
			TopNotice:     5,
		},
		Notices: []CompactRecommend{
			{
				Recommend:      "yes",
				SuggestedAngle: "从 CPA 配置到 grok 注册机集成",
				Why:            []string{"本月多次实质性编辑", "尚未有对应 blog"},
				BlogTitles:     []string{"2026-06-01 — older piece"},
				Topic:          HotTopic{TopicPath: "AI/LLM/model-routing"},
			},
		},
	}
	html, err := RenderCompactHTML(&in)
	require.NoError(t, err)
	require.Contains(t, html, "<h2")
	require.Contains(t, html, "AI/LLM/model-routing")
	require.Contains(t, html, "Angle")
	require.Contains(t, html, "Why")
	require.Contains(t, html, "Existing blogs")
	require.Contains(t, html, "从 CPA 配置到 grok 注册机集成")

	text := RenderCompactText(&in)
	require.Contains(t, text, "## AI/LLM/model-routing")
	require.Contains(t, text, "### Angle")
	require.Contains(t, text, "### Why")
	require.Contains(t, text, "Window: last-month [")
	require.Contains(t, text, "topHot=10")
	require.Contains(t, text, "topNotice=5")
}

func TestRenderCompactHTMLEmptyWithHotTable(t *testing.T) {
	in := CompactMailInput{
		Date:  time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC),
		Since: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Params: CompactParams{
			SinceDuration: "last-month",
			BulkThreshold: 10,
			MinDeltaChars: 40,
			MinDeltaLines: 2,
			TopHot:        10,
			TopNotice:     5,
		},
		HotTopics: []HotTopic{
			{
				TopicPath:   "infra/proxy",
				EditDays:    3,
				EditCommits: 4,
				DeltaChars:  200,
				Score:       42,
				LastEdit:    time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	html, err := RenderCompactHTML(&in)
	require.NoError(t, err)
	require.Contains(t, html, "0 compact notices")
	require.Contains(t, html, "this month")
	require.Contains(t, html, "topic")
	require.Contains(t, html, "days")
	require.Contains(t, html, "commits")
	require.Contains(t, html, "infra/proxy")
	require.Contains(t, html, "<table")

	text := RenderCompactText(&in)
	require.Contains(t, text, "Hot topics · 1")
	require.Contains(t, text, "infra/proxy")
	require.Contains(t, text, "this month")
}

func TestRenderCompactHTMLAISkipped(t *testing.T) {
	in := CompactMailInput{
		Date:      time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC),
		Since:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until:     time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AISkipped: true,
		Params:    CompactParams{SinceDuration: "last-month"},
		HotTopics: []HotTopic{{TopicPath: "a/b", LastEdit: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)}},
	}
	html, err := RenderCompactHTML(&in)
	require.NoError(t, err)
	require.Contains(t, html, "AI unavailable")
	require.Contains(t, html, "a/b")
	require.NotContains(t, html, "AI recommended none")
}

func TestRenderCompactHTMLSkipAI(t *testing.T) {
	in := CompactMailInput{
		Date:      time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC),
		Since:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Until:     time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AISkipped: true,
		SkipAI:    true,
		Params:    CompactParams{SinceDuration: "last-month"},
		HotTopics: []HotTopic{{TopicPath: "a/b", LastEdit: time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)}},
	}
	_, err := RenderCompactHTML(&in)
	require.NoError(t, err)
	// HTML may typographically rewrite "--" (e.g. &ndash;); assert on Markdown body.
	text := RenderCompactText(&in)
	require.Contains(t, text, "--skip-ai")
	require.NotContains(t, text, "AI unavailable")
	require.NotContains(t, text, "AI recommended none")
	require.Contains(t, text, "a/b")
}

func TestRenderCompactPrompt(t *testing.T) {
	prompt, err := prompt.Render("compact.txt", compactPromptData{
		TopicPath:   "AI/LLM/model-routing",
		LogPath:     "wiki/AI/LLM/model-routing/log.md",
		EditDays:    3,
		EditCommits: 4,
		DeltaChars:  200,
		DeltaLines:  10,
		LastEdit:    "2026-07-18 12:00:00",
		Score:       42,
		BlogTitles:  []string{"2026-06-01 — older"},
		Diff:        "+ note",
		LogBody:     "log tail",
	})
	require.NoError(t, err)
	require.Contains(t, prompt, "AI/LLM/model-routing")
	require.Contains(t, prompt, "suggested_angle")
	require.NotContains(t, prompt, "{{", "rendered prompt should not contain template marker")
}

func TestIsSubstantiveViaGitutil(t *testing.T) {
	// smoke: Aggregate empty
	require.Empty(t, AggregateHotTopics(nil, "wiki"))
}
