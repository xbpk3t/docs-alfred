package compact

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/prompt"
	"github.com/xbpk3t/docs-alfred/internal/linear"
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

func TestRunCompactSkipsOutsideWindow(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	// schedule=2, week starting 07-27 (index 29, odd) → out of window.
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, loc)

	res, err := RunCompact(context.Background(), &CompactOptions{
		Now: func() time.Time { return now },
		WindowFn: func(n time.Time) (Window, bool, string) {
			return ScheduleWindow(2, n)
		},
		// RepoRoot points at a non-git temp dir: if the skip guard leaked,
		// git log collection would error and fail the test.
		RepoRoot: t.TempDir(),
		SendMail: true,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Skipped)
	require.Contains(t, res.SkipReason, "outside 2w window")
	require.False(t, res.MailSent)
	require.False(t, res.IssueCreated)
}

func TestScheduleWindowWeekly(t *testing.T) {
	// Asia/Shanghai after carboninit.Setup; pin wall clock mid-July.
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	cases := []struct {
		name string
		now  time.Time
		in   bool
		// expected start/end week boundary (Monday 00:00 Shanghai)
		start, end time.Time
	}{
		{
			name:  "monday",
			now:   time.Date(2026, 7, 13, 9, 0, 0, 0, loc), // Monday
			in:    true,
			start: time.Date(2026, 7, 13, 0, 0, 0, 0, loc), // this Monday
			end:   time.Date(2026, 7, 20, 0, 0, 0, 0, loc), // next Monday
		},
		{
			name:  "midweek",
			now:   time.Date(2026, 7, 15, 12, 0, 0, 0, loc), // Wednesday
			in:    true,
			start: time.Date(2026, 7, 13, 0, 0, 0, 0, loc),
			end:   time.Date(2026, 7, 20, 0, 0, 0, 0, loc),
		},
		{
			name:  "sunday",
			now:   time.Date(2026, 7, 19, 23, 59, 59, 0, loc), // Sunday
			in:    true,
			start: time.Date(2026, 7, 13, 0, 0, 0, 0, loc),
			end:   time.Date(2026, 7, 20, 0, 0, 0, 0, loc),
		},
		{
			name:  "exact boundary",
			now:   time.Date(2026, 7, 13, 0, 0, 0, 0, loc), // Monday 00:00
			in:    true,
			start: time.Date(2026, 7, 13, 0, 0, 0, 0, loc),
			end:   time.Date(2026, 7, 20, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			win, in, reason := ScheduleWindow(1, tc.now)
			require.Equal(t, tc.in, in)
			if in {
				require.Equal(t, "1w", win.Label)
				require.Equal(t, tc.start.Unix(), win.Start.Unix())
				require.Equal(t, tc.end.Unix(), win.End.Unix())
				require.Empty(t, reason)
			} else {
				require.Contains(t, reason, "outside")
			}
		})
	}
}

func TestScheduleWindowBiweekly(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	// weekIndex anchor: 2026-01-05 (Monday). Weeks are indexed 0,1,2,…
	//   2026-07-20 → index 28 (even), 2026-07-27 → index 29 (odd),
	//   2026-08-03 → index 30 (even).
	// schedule=2 triggers when weekIndex%2==0 → week 2026-07-20 and
	// 2026-08-03 trigger; week 2026-07-27 skips.
	inWin := time.Date(2026, 8, 5, 10, 0, 0, 0, loc) // Thu in week starting 08-03 (even, in)
	win, in, reason := ScheduleWindow(2, inWin)
	require.True(t, in)
	require.Empty(t, reason)
	require.Equal(t, "2w", win.Label)
	// window = [07-27 00:00, 08-10 00:00) Shanghai (prev + current week).
	require.Equal(t, time.Date(2026, 7, 27, 0, 0, 0, 0, loc).Unix(), win.Start.Unix())
	require.Equal(t, time.Date(2026, 8, 10, 0, 0, 0, 0, loc).Unix(), win.End.Unix())
	require.Equal(t, 14*24*time.Hour, win.End.Sub(win.Start))

	outWin := time.Date(2026, 7, 29, 10, 0, 0, 0, loc) // Wed in week starting 07-27 (odd, out)
	_, in, reason = ScheduleWindow(2, outWin)
	require.False(t, in)
	require.Contains(t, reason, "outside 2w window")
}

func TestScheduleWindowDefaultsToOne(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	win, in, _ := ScheduleWindow(0, now)
	require.True(t, in)
	require.Equal(t, "1w", win.Label)
	require.Equal(t, 7*24*time.Hour, win.End.Sub(win.Start))
}

func TestRenderCompactIssueTitle(t *testing.T) {
	day := time.Date(2026, 8, 1, 5, 0, 0, 0, time.FixedZone("CST", 8*3600))
	require.Equal(t, "wiki compact [2026-08-01]", RenderCompactIssueTitle("", day))
	require.Equal(t, "wiki compact [2026-08-01]", RenderCompactIssueTitle("wiki compact", day))
	require.Equal(t, "my brand [2026-08-01]", RenderCompactIssueTitle("my brand", day))
}

type fakeLinearCreator struct {
	teamID   string
	stateID  string
	viewerID string
	last     linear.CreateIssueInput
	err      error
}

func (f *fakeLinearCreator) CreateIssue(_ context.Context, in *linear.CreateIssueInput) (*linear.Issue, error) {
	if in == nil {
		return nil, fmt.Errorf("nil input")
	}
	f.last = *in
	if f.err != nil {
		return nil, f.err
	}
	return &linear.Issue{
		ID:         "id-1",
		Identifier: "LUC-100",
		Title:      in.Title,
		URL:        "https://linear.app/luckzzz/issue/LUC-100",
		Priority:   float64(in.Priority),
		StateName:  "In Review",
		StateType:  "started",
	}, nil
}

func (f *fakeLinearCreator) ResolveTeamID(_ context.Context, teamKey string) (string, error) {
	if f.teamID != "" {
		return f.teamID, nil
	}
	return "team-" + teamKey, nil
}

func (f *fakeLinearCreator) ResolveStateID(_ context.Context, teamID, stateName string) (string, error) {
	if f.stateID != "" {
		return f.stateID, nil
	}
	return "state-" + stateName, nil
}

func (f *fakeLinearCreator) ViewerID(_ context.Context) (string, error) {
	if f.viewerID != "" {
		return f.viewerID, nil
	}
	return "viewer-1", nil
}

func TestCreateCompactIssue_UsesTextBodyAndTitle(t *testing.T) {
	fake := &fakeLinearCreator{teamID: "team-luc", stateID: "state-review", viewerID: "user-me"}
	cfg := &LinearConfig{
		APIKey:    "k",
		TeamKey:   "LUC",
		StateName: "In Review",
		Priority:  2,
		Assignee:  "viewer",
		NewClient: func(apiKey string, teamKeys []string) LinearIssueCreator {
			return fake
		},
	}
	issue, err := CreateCompactIssue(context.Background(), cfg, "wiki compact [2026-08-01]", "hello body")
	require.NoError(t, err)
	require.Equal(t, "LUC-100", issue.Identifier)
	require.Equal(t, "team-luc", fake.last.TeamID)
	require.Equal(t, "wiki compact [2026-08-01]", fake.last.Title)
	require.Equal(t, "hello body", fake.last.Description)
	require.Equal(t, "state-review", fake.last.StateID)
	require.Equal(t, "user-me", fake.last.AssigneeID)
	require.Equal(t, 2, fake.last.Priority)
}

func TestCreateCompactIssue_AssigneeNone(t *testing.T) {
	fake := &fakeLinearCreator{teamID: "team-luc", stateID: "state-review"}
	cfg := &LinearConfig{
		APIKey:    "k",
		TeamID:    "team-luc",
		StateName: "In Review",
		Priority:  2,
		Assignee:  "none",
		NewClient: func(apiKey string, teamKeys []string) LinearIssueCreator {
			return fake
		},
	}
	_, err := CreateCompactIssue(context.Background(), cfg, "t", "b")
	require.NoError(t, err)
	require.Empty(t, fake.last.AssigneeID)
}

func TestCreateCompactIssue_RequiresAPIKey(t *testing.T) {
	_, err := CreateCompactIssue(context.Background(), &LinearConfig{}, "t", "b")
	require.Error(t, err)
	require.Contains(t, err.Error(), "api key")
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
	require.True(t, strings.HasPrefix(RenderCompactSubject(&CompactMailInput{Date: day}), "[wiki compact]"))
	require.True(t, strings.HasPrefix(RenderCompactSubject(&CompactMailInput{Date: day, Title: "acme"}), "[acme]"))
}

func TestCompactBrand(t *testing.T) {
	require.Equal(t, DefaultBrand, CompactBrand(""))
	require.Equal(t, DefaultBrand, CompactBrand("  "))
	require.Equal(t, "acme", CompactBrand(" acme "))
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
	require.Contains(t, html, "this window")
	require.Contains(t, html, "topic")
	require.Contains(t, html, "days")
	require.Contains(t, html, "commits")
	require.Contains(t, html, "infra/proxy")
	require.Contains(t, html, "<table")

	text := RenderCompactText(&in)
	require.Contains(t, text, "Hot topics · 1")
	require.Contains(t, text, "infra/proxy")
	require.Contains(t, text, "this window")
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
