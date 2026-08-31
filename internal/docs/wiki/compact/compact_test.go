package compact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/linear"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

func init() {
	carboninit.Setup()
	validator.Setup()
}

const researchFM = "---\ntype: research\ntitle: t\ndate: 2026-01-01\nsource: test\n---\n\nbody\n"

// makeTempWiki builds a temp wiki tree for compact tests.
func makeTempWiki(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	return root
}

func alwaysWindow(time.Time) (bool, string) { return true, "" }

// ---------- schedule / window ----------

func TestRunCompactSkipsOutsideWindow(t *testing.T) {
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.FixedZone("CST", 8*3600)) // Wed
	res, err := RunCompact(context.Background(), &CompactOptions{
		Now:         func() time.Time { return now },
		WindowFn:    func(n time.Time) (bool, string) { return ScheduleWindow(1, n) },
		SendMail:    true,
		CreateIssue: true,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Skipped)
	require.Contains(t, res.SkipReason, "not schedule day")
	require.False(t, res.MailSent)
	require.False(t, res.IssueCreated)
}

func TestScheduleWindowWeekly(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	// Sat 07-18 fires weekly.
	in, reason := ScheduleWindow(1, time.Date(2026, 7, 18, 9, 0, 0, 0, loc))
	require.True(t, in)
	require.Empty(t, reason)

	// All non-Saturday weekdays skip.
	for _, day := range []time.Time{
		time.Date(2026, 7, 13, 9, 0, 0, 0, loc),    // Mon
		time.Date(2026, 7, 15, 12, 0, 0, 0, loc),   // Wed
		time.Date(2026, 7, 19, 23, 59, 59, 0, loc), // Sun
	} {
		in, reason = ScheduleWindow(1, day)
		require.False(t, in, day.Weekday().String())
		require.Contains(t, reason, "not schedule day")
	}
}

func TestScheduleWindowBiweekly(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	// Sat 07-25 (even week index) fires biweekly.
	in, reason := ScheduleWindow(2, time.Date(2026, 7, 25, 9, 0, 0, 0, loc))
	require.True(t, in)
	require.Empty(t, reason)

	// Sat 08-01 (odd week) not eligible; Wed 07-22 not the schedule day.
	in, reason = ScheduleWindow(2, time.Date(2026, 8, 1, 9, 0, 0, 0, loc))
	require.False(t, in)
	require.Contains(t, reason, "week not eligible")
	in, reason = ScheduleWindow(2, time.Date(2026, 7, 22, 9, 0, 0, 0, loc))
	require.False(t, in)
	require.Contains(t, reason, "not schedule day")
}

func TestScheduleWindowDefaultsToOne(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	// schedule=0 → 1: Saturday fires, Wednesday skips.
	in, _ := ScheduleWindow(0, time.Date(2026, 7, 18, 9, 0, 0, 0, loc))
	require.True(t, in)
	in, _ = ScheduleWindow(0, time.Date(2026, 7, 15, 9, 0, 0, 0, loc))
	require.False(t, in)
}

// ---------- 清零 ranking / RunCompact ----------

func TestRunCompact_ListsRankedCandidates(t *testing.T) {
	root := makeTempWiki(t, map[string]string{
		"AI/go/a.md": researchFM, // research
		"AI/go/b.md": researchFM, // research → AI/go has 2 research
		"sys/x.md":   researchFM, // sys has 1 research
	})
	now := time.Date(2026, 8, 1, 5, 0, 0, 0, time.FixedZone("CST", 8*3600)) // Sat
	res, err := RunCompact(context.Background(), &CompactOptions{
		WikiRoot: root,
		Now:      func() time.Time { return now },
		WindowFn: alwaysWindow,
		TopN:     2,
		Title:    "acme",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.Skipped)
	require.Len(t, res.Candidates, 2)
	// Alphabetical tie-break isn't needed: AI/go (2 research) ranks above sys (1).
	require.Equal(t, "AI/go", res.Candidates[0].Path)
	require.Equal(t, 1, res.Candidates[0].Rank)
	require.Equal(t, 2, res.Candidates[0].Research)
	require.Equal(t, "sys", res.Candidates[1].Path)
	require.Equal(t, 2, res.Candidates[1].Rank)

	require.Contains(t, res.Subject, "2 candidate(s)")
	require.Contains(t, res.TextBody, "AI/go")
}

func TestRunCompact_EmptyCensusSkips(t *testing.T) {
	// jsonl is an excluded artifact → no md content in the census.
	root := makeTempWiki(t, map[string]string{"digest.jsonl": "{}\n"})
	res, err := RunCompact(context.Background(), &CompactOptions{
		WikiRoot: root,
		Now:      func() time.Time { return time.Date(2026, 8, 1, 5, 0, 0, 0, time.UTC) },
		WindowFn: alwaysWindow,
	})
	require.NoError(t, err)
	require.True(t, res.Skipped)
	require.Contains(t, res.SkipReason, "清零")
}

func TestRunCompact_DeliversMailAndIssue(t *testing.T) {
	root := makeTempWiki(t, map[string]string{"AI/go/a.md": researchFM})
	fake := &fakeLinearCreator{teamID: "team-luc", stateID: "state-review", viewerID: "user-me"}
	res, err := RunCompact(context.Background(), &CompactOptions{
		WikiRoot:    root,
		Now:         func() time.Time { return time.Date(2026, 8, 1, 5, 0, 0, 0, time.UTC) },
		WindowFn:    alwaysWindow,
		TopN:        10,
		SendMail:    false, // avoid real send; issue via fake NewClient
		CreateIssue: true,
		Linear: LinearConfig{
			APIKey:    "k",
			TeamKey:   "LUC",
			StateName: "In Review",
			Priority:  2,
			Assignee:  "viewer",
			NewClient: func(apiKey string, teamKeys []string) LinearIssueCreator { return fake },
		},
	})
	require.NoError(t, err)
	require.True(t, res.IssueCreated)
	require.Equal(t, "LUC-100", res.IssueIdentifier)
	require.Equal(t, "team-luc", fake.last.TeamID)
	require.Contains(t, fake.last.Description, "AI/go")
}

func TestNormalizeCompactOptsDefaults(t *testing.T) {
	opts := &CompactOptions{}
	normalizeCompactOpts(opts)
	require.Equal(t, 10, opts.TopN)
}

// ---------- mail rendering ----------

func TestRenderCompactSubject(t *testing.T) {
	day := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	require.Contains(t, RenderCompactSubject(&CompactMailInput{Date: day}), "none")
	require.Contains(t, RenderCompactSubject(&CompactMailInput{Date: day, Candidates: []ZeroCandidate{{}, {}}}), "2 candidate(s)")
	require.True(t, strings.HasPrefix(RenderCompactSubject(&CompactMailInput{Date: day}), "[wiki compact]"))
	require.True(t, strings.HasPrefix(RenderCompactSubject(&CompactMailInput{Date: day, Title: "acme"}), "[acme]"))
}

func TestCompactBrand(t *testing.T) {
	require.Equal(t, DefaultBrand, CompactBrand(""))
	require.Equal(t, DefaultBrand, CompactBrand("  "))
	require.Equal(t, "acme", CompactBrand(" acme "))
}

func TestRenderCompactHTMLWithCandidates(t *testing.T) {
	in := CompactMailInput{
		Date:  time.Date(2026, 7, 1, 5, 0, 0, 0, time.UTC),
		Title: "acme",
		Candidates: []ZeroCandidate{
			{Path: "AI/LLM", Files: 3, Size: 204800, Research: 2, Score: 12, Rank: 1},
		},
	}
	html, err := RenderCompactHTML(&in)
	require.NoError(t, err)
	require.Contains(t, html, "AI/LLM")
	require.Contains(t, html, "200.0 KiB")

	text := RenderCompactText(&in)
	require.Contains(t, text, "清零 · candidates")
	require.Contains(t, text, "AI/LLM")
	require.Contains(t, text, "Soft reminder")
}

// ---------- linear delivery ----------

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
		NewClient: func(apiKey string, teamKeys []string) LinearIssueCreator { return fake },
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
		NewClient: func(apiKey string, teamKeys []string) LinearIssueCreator { return fake },
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
