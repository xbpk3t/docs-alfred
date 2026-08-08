package compact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/blog"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/gitutil"
)

// Window is a half-open commit time range [Start, End) used for hot detection.
// End zero means open-ended.
type Window struct {
	Start time.Time
	End   time.Time
	// Label is a human/params token: "2w", "1w", …
	Label string
}

// CompactOptions controls docs-cli wiki compact.
type CompactOptions struct {
	Now      func() time.Time
	AI       *ai.ClientConfig
	WindowFn func(now time.Time) (Window, bool, string)
	RepoRoot string
	WikiRoot string
	// Title is compact brand (From / subject / issue title); empty → DefaultBrand.
	Title            string
	Mail             MailConfig
	Linear           LinearConfig
	MinDeltaChars    int
	MinDeltaLines    int
	BulkLogThreshold int
	TopNotice        int
	TopHot           int
	SendMail         bool
	CreateIssue      bool
	DryRun           bool
	SkipAI           bool
}

// CompactResult is the pipeline outcome.
type CompactResult struct {
	Until           time.Time
	Since           time.Time
	SoftError       error
	IssueTitle      string
	IssueURL        string
	Subject         string
	TextBody        string
	HTMLBody        string
	SkipReason      string
	IssueIdentifier string
	HotTopics       []HotTopic
	Judged          []CompactRecommend
	Notices         []CompactRecommend
	AIFailures      int
	Skipped         bool
	AISkipped       bool
	MailSent        bool
	IssueCreated    bool
}

// RunCompact executes hot detect → AI → optional Resend and/or Linear issue create.
func RunCompact(ctx context.Context, opts *CompactOptions) (*CompactResult, error) {
	if opts == nil {
		opts = &CompactOptions{}
	}
	normalizeCompactOpts(opts)
	now := carbon.Now().StdTime()
	if opts.Now != nil {
		now = opts.Now()
	}

	win, inWindow, skipReason := opts.WindowFn(now)
	if !inWindow {
		return &CompactResult{
			Since:      win.Start,
			Until:      win.End,
			Skipped:    true,
			SkipReason: skipReason,
		}, nil
	}

	repoRoot, wikiRel, err := resolveRepoAndWiki(opts)
	if err != nil {
		return nil, err
	}

	edits, err := gitutil.CollectLogEdits(repoRoot, &gitutil.CollectLogEditOptions{
		Since:            win.Start,
		Until:            win.End,
		BulkLogThreshold: opts.BulkLogThreshold,
		MinDeltaChars:    opts.MinDeltaChars,
		MinDeltaLines:    opts.MinDeltaLines,
		PathPrefix:       wikiRel,
	})
	if err != nil {
		return nil, fmt.Errorf("collect log edits: %w", err)
	}

	allHot := AggregateHotTopics(edits, wikiRel)
	// Resolve absolute topic dirs for blog/AI file reads.
	for i := range allHot {
		allHot[i].TopicDir = filepath.Join(repoRoot, filepath.FromSlash(allHot[i].TopicDir))
	}
	hot := TopNHot(allHot, opts.TopHot)

	result := &CompactResult{
		Since:     win.Start,
		Until:     win.End,
		HotTopics: hot,
	}

	judged, aiSkipped, aiFailures := judgeTopics(ctx, opts, hot, win.Start)
	result.Judged = judged
	result.AISkipped = aiSkipped
	result.AIFailures = aiFailures
	if !aiSkipped {
		result.Notices = SelectNotices(judged, opts.TopNotice)
	}

	if err := fillMailBodies(result, opts, win, now, hot, aiSkipped, aiFailures); err != nil {
		return result, err
	}

	if aiSkipped && !opts.SkipAI {
		result.SoftError = fmt.Errorf("AI unavailable; hot list prepared")
	}

	if err := deliverCompact(ctx, opts, result, now); err != nil {
		return result, err
	}

	return result, nil
}

// SkipReasonWindow reports why now falls outside the schedule window.
// Not used directly by RunCompact (that uses WindowFn); kept for callers
// that need a reason before constructing options.
func SkipReasonWindow(schedule int, now time.Time) string {
	day := now.Weekday()
	if day != DefaultScheduleDay {
		return fmt.Sprintf("today is %s, not schedule day %s (schedule=%d)", day, DefaultScheduleDay, schedule)
	}
	return fmt.Sprintf("week not eligible: weekIndex%%%d != 0 (schedule=%d)", schedule, schedule)
}

// DefaultScheduleDay is the weekday on which a compact run fires
// (Saturday). Runs on other days are skipped even in an eligible week.
const DefaultScheduleDay = time.Saturday

// ScheduleWindow returns the current schedule window [start, end) and whether
// now falls inside it.
//
// A run fires only when BOTH hold:
//   - today is the schedule day (DefaultScheduleDay = Saturday), and
//   - the current week is eligible: weekIndex(mondayOf(now)) % schedule == 0.
//
// With schedule=1 that is every Saturday (weekly); with schedule=2 every
// second Saturday (biweekly). actions may trigger daily — any other day is
// skipped with zero side effects.
//
// The window covers `schedule` full weeks ending at 00:00 of the fire day:
// [fireDay - schedule*7d, fireDay). With schedule=1 the window is the single
// week before the fire day; with schedule=2 it spans the previous two weeks.
// Consecutive fire windows are contiguous and non-overlapping: the next run's
// Start equals this run's End, so no commits are dropped or double-counted
// between fire days.
//
// Week index uses the time.Weekday (Monday) of now's wall-clock location — no
// carbon global state is touched. It is an integer day-count from the Monday
// anchor 2026-01-05, taken as calendar dates (timezone-independent, DST-free).
func ScheduleWindow(schedule int, now time.Time) (Window, bool, string) {
	if schedule <= 0 {
		schedule = 1
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	win := Window{
		Start: dayStart.Add(-time.Duration(schedule) * 7 * 24 * time.Hour),
		End:   dayStart,
		Label: strconv.Itoa(schedule) + "w",
	}

	if now.Weekday() == DefaultScheduleDay && weekIndex(mondayOf(now))%schedule == 0 {
		return win, true, ""
	}
	return Window{Label: win.Label}, false, SkipReasonWindow(schedule, now)
}

// mondayAnchor is the Monday 2026-01-05 (calendar date; zone ignored).
var mondayAnchor = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

// mondayOf returns the Monday 00:00 of now's week, keeping now's location
// (Asia/Shanghai after carboninit.Setup). Pure weekday arithmetic — does not
// read or mutate carbon's global week-start setting.
func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // days since Monday
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, -offset)
}

// weekIndex returns the count of whole weeks between the mondayAnchor date and
// the curStart date, using calendar dates only (zone-free).
func weekIndex(curStart time.Time) int {
	aY, aM, aD := mondayAnchor.Date()
	cY, cM, cD := curStart.Date()
	anchor := time.Date(aY, aM, aD, 0, 0, 0, 0, time.UTC)
	cur := time.Date(cY, cM, cD, 0, 0, 0, 0, time.UTC)
	days := int(cur.Sub(anchor).Hours() / 24)
	if days < 0 {
		return days / 7
	}
	return days / 7
}

// deliverCompact sends optional Resend mail and/or creates a Linear issue.
// Channels are independent: one failure does not roll back the other.
func deliverCompact(ctx context.Context, opts *CompactOptions, result *CompactResult, now time.Time) error {
	var deliverErr error

	if opts.SendMail && !opts.DryRun {
		if err := SendCompactEmail(ctx, &opts.Mail, opts.Title, result.Subject, result.HTMLBody); err != nil {
			deliverErr = fmt.Errorf("send mail: %w", err)
		} else {
			result.MailSent = true
		}
	}

	if !opts.CreateIssue {
		return deliverErr
	}

	result.IssueTitle = RenderCompactIssueTitle(opts.Title, now)
	if opts.DryRun {
		return deliverErr
	}

	issue, err := CreateCompactIssue(ctx, &opts.Linear, result.IssueTitle, result.TextBody)
	if err != nil {
		if deliverErr != nil {
			return fmt.Errorf("%w; create linear issue: %w", deliverErr, err)
		}
		return fmt.Errorf("create linear issue: %w", err)
	}
	if issue != nil {
		result.IssueCreated = true
		result.IssueIdentifier = issue.Identifier
		result.IssueURL = issue.URL
	}
	return deliverErr
}

func resolveRepoAndWiki(opts *CompactOptions) (repoRoot, wikiRel string, err error) {
	repoRoot = opts.RepoRoot
	if repoRoot == "" {
		wd, werr := os.Getwd()
		if werr != nil {
			return "", "", werr
		}
		repoRoot, werr = gitutil.FindRepoRoot(wd)
		if werr != nil {
			return "", "", fmt.Errorf("find repo root: %w", werr)
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}

	wikiRoot := opts.WikiRoot
	if wikiRoot == "" {
		wikiRoot = "wiki"
	}
	wikiAbs := wikiRoot
	if !filepath.IsAbs(wikiAbs) {
		wikiAbs = filepath.Join(repoRoot, wikiRoot)
	}
	wikiRel, err = filepath.Rel(repoRoot, wikiAbs)
	if err != nil {
		wikiRel = wikiRoot
	}
	return repoRoot, filepath.ToSlash(wikiRel), nil
}

func judgeTopics(
	ctx context.Context,
	opts *CompactOptions,
	hot []HotTopic,
	winStart time.Time,
) (judged []CompactRecommend, aiSkipped bool, aiFailures int) {
	if opts.SkipAI {
		// Intentional offline/debug: keep AISkipped so mail/subject say "AI skipped",
		// not "AI recommended none". SoftError is not set by caller.
		for i := range hot {
			judged = append(judged, CompactRecommend{
				Topic:     hot[i],
				Recommend: "no",
				Why:       []string{"AI skipped (--skip-ai or offline)"},
			})
		}
		return judged, true, 0
	}

	toJudge := make([]HotTopic, 0, len(hot))
	for i := range hot {
		ht := hot[i]
		hasNew, _, coolErr := blog.TopicHasNewBlogInWindow(ht.TopicDir, winStart)
		if coolErr == nil && hasNew {
			judged = append(judged, CompactRecommend{
				Topic:          ht,
				Recommend:      "no",
				SkippedCooling: true,
				Why:            []string{"skipped: new type:blog in window"},
			})
			continue
		}
		toJudge = append(toJudge, ht)
	}

	if len(toJudge) == 0 {
		return judged, false, 0
	}

	aiCfg := opts.AI
	if aiCfg == nil {
		aiCfg = ai.DefaultConfig()
	}
	if aiCfg.APIKey == "" {
		return judged, true, 0
	}

	part, ok := JudgeHotTopics(ctx, aiCfg, toJudge)
	for i := range part {
		if part[i].Error != "" {
			aiFailures++
		}
	}
	judged = append(judged, part...)
	if !ok {
		return judged, true, aiFailures
	}
	return judged, false, aiFailures
}

func fillMailBodies(
	result *CompactResult,
	opts *CompactOptions,
	win Window,
	now time.Time,
	hot []HotTopic,
	aiSkipped bool,
	aiFailures int,
) error {
	params := CompactParams{
		SinceDuration: win.Label,
		BulkThreshold: opts.BulkLogThreshold,
		MinDeltaChars: opts.MinDeltaChars,
		MinDeltaLines: opts.MinDeltaLines,
		TopHot:        opts.TopHot,
		TopNotice:     opts.TopNotice,
	}
	mailIn := CompactMailInput{
		Date:       now,
		Since:      win.Start,
		Until:      win.End,
		Notices:    result.Notices,
		HotTopics:  hot,
		Title:      opts.Title,
		AISkipped:  aiSkipped,
		SkipAI:     opts.SkipAI,
		AIFailures: aiFailures,
		Params:     params,
	}
	result.Subject = RenderCompactSubject(&mailIn)
	htmlBody, err := RenderCompactHTML(&mailIn)
	if err != nil {
		return fmt.Errorf("render compact HTML: %w", err)
	}
	result.HTMLBody = htmlBody
	result.TextBody = RenderCompactText(&mailIn)
	return nil
}

func normalizeCompactOpts(opts *CompactOptions) {
	if opts.TopHot <= 0 {
		opts.TopHot = 10
	}
	if opts.TopNotice <= 0 {
		opts.TopNotice = 5
	}
	if opts.BulkLogThreshold <= 0 {
		opts.BulkLogThreshold = 10
	}
	if opts.MinDeltaChars <= 0 {
		opts.MinDeltaChars = 40
	}
	if opts.MinDeltaLines <= 0 {
		opts.MinDeltaLines = 2
	}
}
