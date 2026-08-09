package compact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	AI               *ai.ClientConfig
	WindowFn         func(now time.Time) (Window, bool, string)
	Now              func() time.Time
	RepoRoot         string
	WikiRoot         string
	Title            string
	Mail             MailConfig
	Linear           LinearConfig
	Gate             CompactGate
	MinDeltaChars    int
	BulkLogThreshold int
	MinDeltaLines    int
	MinGateAge       time.Duration
	TopHot           int
	SendMail         bool
	CreateIssue      bool
	DryRun           bool
	SkipAI           bool
}

// RunCompact is the entry point for the compact pipeline.
type CompactResult struct {
	WallClockNow     time.Time
	Since            time.Time
	Until            time.Time
	SoftError        error
	IssueTitle       string
	IssueURL         string
	Subject          string
	TextBody         string
	HTMLBody         string
	SkipReason       string
	IssueIdentifier  string
	HotTopics        []HotTopic
	Notices          []CompactRecommend
	JudgedHeat       []HotTopic
	Rejected         []HotTopic
	Judged           []CompactRecommend
	Gate             CompactGate
	AIFailures       int
	Skipped          bool
	SkippedEmptyGate bool
	AISkipped        bool
	MailSent         bool
	IssueCreated     bool
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

	gate := gateForWindow(opts, win.Start, now)
	admitted, rejected := applyGate(gate, allHot)
	hot := TopNHot(admitted, opts.TopHot)

	result := &CompactResult{
		Since:        win.Start,
		Until:        win.End,
		HotTopics:    hot,
		Gate:         gate,
		WallClockNow: now,
		Rejected:     rejected,
	}
	if len(hot) == 0 {
		return result, emptyGateResult(result, gate, win, now, rejected)
	}
	result.JudgedHeat = hot

	judged, aiSkipped, aiFailures := judgeTopics(ctx, opts, hot, win.Start)
	result.Judged = judged
	result.AISkipped = aiSkipped
	result.AIFailures = aiFailures

	// Hard gate: a topic whose AI judgement is "yes" but duplicates an existing
	// blog slice is NOT admitted — fact, not counted. Empty input → skip delivery.
	result.Notices = admittedNotices(judged)
	if result.Notices == nil && !aiSkipped && !opts.SkipAI {
		return result, noNoticesResult(result, opts, win, now, hot, aiSkipped, aiFailures)
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

// gateForWindow materializes the heat gate with its decay window clamped to
// the run window (decayed topics below the gate are rejected).
func gateForWindow(opts *CompactOptions, winStart, now time.Time) CompactGate {
	gate := defaultGate(opts, winStart)
	decaySince := now.Add(-opts.MinGateAge)
	if opts.MinGateAge <= 0 || decaySince.Before(winStart) {
		decaySince = winStart
	}
	gate.DecaySince = decaySince
	return gate
}

// applyGate partitions topics into admitted (heat-passed) and rejected, and
// records GatePassed/Reasons on each for the transparency table.
func applyGate(gate CompactGate, topics []HotTopic) (admitted, rejected []HotTopic) {
	admitted = make([]HotTopic, 0, len(topics))
	rejected = make([]HotTopic, 0, len(topics))
	for i := range topics {
		pass, reasons := gate.PassesCompactGate(&topics[i])
		topics[i].GatePassed = pass
		topics[i].Reasons = reasons
		if pass {
			admitted = append(admitted, topics[i])
		} else {
			rejected = append(rejected, topics[i])
		}
	}
	return admitted, rejected
}

// noNoticesResult marks the run skipped when AI judged nothing worth writing.
func noNoticesResult(result *CompactResult, opts *CompactOptions, win Window, now time.Time,
	hot []HotTopic, aiSkipped bool, aiFailures int) error {
	result.Skipped = true
	if err := fillMailBodies(result, opts, win, now, hot, aiSkipped, aiFailures); err != nil {
		return err
	}
	result.SkipReason = "all hot topics were judged no (or duplicate) — nothing to compact"
	return nil
}

// emptyGateResult fills the already-constructed result so the heat table is
// still visible (the rejected list with reasons) and the run is marked skipped.
func emptyGateResult(result *CompactResult, gate CompactGate, win Window,
	now time.Time, rejected []HotTopic) error {
	result.HotTopics = rejected
	result.Rejected = rejected
	result.Skipped = true
	result.SkippedEmptyGate = true
	if err := fillMailBodies(result, &CompactOptions{}, win, now, rejected, false, 0); err != nil {
		return err
	}
	result.SkipReason = fmt.Sprintf("no topic cleared the heat gate (days≥%d ∧ commits≥%d ∨ Δchars≥%d)",
		gate.MinEditDays, gate.MinEditCommits, gate.MinDeltaChars)
	return nil
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

// fillMailBodies renders subject/HTML/text into result. `hot` is only used as
// the fallback heat list for the "0 admitted" case (rejected carries reasons).
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
		Gate:          result.Gate,
	}
	mailIn := CompactMailInput{
		Date:       now,
		Since:      win.Start,
		Until:      win.End,
		Notices:    result.Notices,
		HotTopics:  hot,
		Heat:       result.JudgedHeat,
		Rejected:   result.Rejected,
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

// defaultGate materializes the heat gate defaults in one place.
func defaultGate(opts *CompactOptions, winStart time.Time) CompactGate {
	g := CompactGate{
		// B1 + strictest delta bucket: sustained editing (≥2 distinct days ∧ ≥2
		// distinct commits) OR one material append (≥2000 non-whitespace chars).
		MinEditDays:    2,
		MinEditCommits: 2,
		MinDeltaChars:  2000,
		DecaySince:     winStart,
	}
	// Caller overrides are honored where set (>0), keeping defaults centralized.
	if opts.Gate.MinEditDays > 0 {
		g.MinEditDays = opts.Gate.MinEditDays
	}
	if opts.Gate.MinEditCommits > 0 {
		g.MinEditCommits = opts.Gate.MinEditCommits
	}
	if opts.Gate.MinDeltaChars > 0 {
		g.MinDeltaChars = opts.Gate.MinDeltaChars
	}
	return g
}

// admittedNotices applies the final hard gates to AI judgements:
//  1. cooling: a new type:blog authored in-window for the same topic → exclude.
//  2. duplicate: AI said yes but named an existing blog it duplicates → no.
//
// Returns in input order (already score-sorted). Everything that survives is
// delivered — no top-N cut, so when this list is empty the run is skipped.
func admittedNotices(judged []CompactRecommend) []CompactRecommend {
	out := make([]CompactRecommend, 0, len(judged))
	for i := range judged {
		r := &judged[i]
		if r.SkippedCooling || !strings.EqualFold(r.Recommend, "yes") {
			continue
		}
		if strings.TrimSpace(r.DuplicateOf) != "" {
			// Hard gate: fact, not opinion — a yes that names an existing blog
			// slice must not be delivered. The rejection stays visible in
			// result.Judged; the mail heat table shows the topic as pass.
			continue
		}
		out = append(out, *r)
	}
	return out
}
