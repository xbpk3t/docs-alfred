package compact

import (
	"context"
	"fmt"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/audit"
	"github.com/xbpk3t/docs-alfred/pkg/rankings"
)

// CompactOptions controls docs-cli wiki compact.
type CompactOptions struct {
	WindowFn    func(now time.Time) (inWindow bool, reason string)
	Now         func() time.Time
	WikiRoot    string
	Title       string
	Mail        MailConfig
	Linear      LinearConfig
	TopN        int
	SendMail    bool
	CreateIssue bool
	DryRun      bool
}

// CompactResult is the output of a compact (清零) run.
type CompactResult struct {
	IssueTitle      string
	IssueURL        string
	Subject         string
	TextBody        string
	HTMLBody        string
	SkipReason      string
	IssueIdentifier string
	Candidates      []ZeroCandidate
	Skipped         bool
	MailSent        bool
	IssueCreated    bool
}

// ZeroCandidate is one ranked 清零 candidate (a whole-wiki topic) surfaced by
// the run. Score is the deterministic census ranking; Rank is 1-based.
type ZeroCandidate struct {
	Path     string
	Files    int
	Size     int64
	Research int
	Score    float64
	Rank     int
}

// RunCompact executes the weekly 清零 pipeline:
//
//	schedule gate (fire only on the configured day/week) →
//	full-state census (audit.TopicMetrics) →
//	deterministic score + rank via pkg/rankings →
//	top-N candidates → optional Resend and/or Linear delivery.
//
// No AI, no git log, no log.md — the batch is the entire current wiki state,
// because 每周清零 resets each run. The run never writes blog or log.md.
func RunCompact(ctx context.Context, opts *CompactOptions) (*CompactResult, error) {
	if opts == nil {
		opts = &CompactOptions{}
	}
	normalizeCompactOpts(opts)
	now := carbon.Now().StdTime()
	if opts.Now != nil {
		now = opts.Now()
	}

	inWindow, skipReason := opts.WindowFn(now)
	if !inWindow {
		return &CompactResult{Skipped: true, SkipReason: skipReason}, nil
	}

	metrics, err := audit.TopicMetrics(opts.WikiRoot)
	if err != nil {
		return nil, fmt.Errorf("compact census: %w", err)
	}

	candidates, err := rankCandidates(ctx, opts, metrics)
	if err != nil {
		return nil, err
	}

	result := &CompactResult{Candidates: candidates}
	if len(candidates) == 0 {
		result.Skipped = true
		result.SkipReason = "no topic in the census cleared 清零 (empty top-N)"
	}
	if err := fillMailBodies(result, opts, now); err != nil {
		return result, err
	}

	if err := deliverCompact(ctx, opts, result, now); err != nil {
		return result, err
	}
	return result, nil
}

// rankCandidates scores every topic from the census and returns the top-N as
// 清零 candidates, ranked desc. Metrics are re-indexed by path to enrich the
// returned candidates (RankItem only carries member/score/rank).
func rankCandidates(ctx context.Context, opts *CompactOptions, metrics []audit.TopicMetric) ([]ZeroCandidate, error) {
	if len(metrics) == 0 {
		return nil, nil
	}
	storage := rankings.NewMemStorage()
	rk := rankings.NewRanking("wiki-zero", "zero",
		WikiCensusScoreCalc{}, &rankings.NoDecay{},
		&rankings.AllTimeWindow{Name: "wiki:all"}, storage)

	// Index metrics by path in the same pass that submits them, so ranked
	// RankItems (member/score/rank only) can be enriched back to candidates.
	byPath := make(map[string]audit.TopicMetric, len(metrics))
	for _, m := range metrics {
		byPath[m.Path] = m
		if err := rk.Submit(ctx, censusItem(m)); err != nil {
			return nil, err
		}
	}
	top, err := rk.TopN(ctx, opts.TopN)
	if err != nil {
		return nil, err
	}

	candidates := make([]ZeroCandidate, 0, len(top))
	for _, r := range top {
		m := byPath[r.Member]
		candidates = append(candidates, ZeroCandidate{
			Path:     m.Path,
			Files:    m.Files,
			Size:     m.Size,
			Research: m.Research,
			Score:    r.Score,
			Rank:     r.Rank,
		})
	}
	return candidates, nil
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

func normalizeCompactOpts(opts *CompactOptions) {
	if opts.TopN <= 0 {
		opts.TopN = 10
	}
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

// ScheduleWindow reports whether now is a fire day (Saturday in an eligible
// week) and, when skipped, why.
//
// A run fires only when BOTH hold:
//   - today is the schedule day (DefaultScheduleDay = Saturday), and
//   - the current week is eligible: weekIndex(mondayOf(now)) % schedule == 0.
//
// With schedule=1 that is every Saturday (weekly); with schedule=2 every
// second Saturday (biweekly). actions may trigger daily — any other day is
// skipped with zero side effects.
//
// Week index uses the time.Weekday (Monday) of now's wall-clock location — no
// carbon global state is touched. It is an integer day-count from the Monday
// anchor 2026-01-05, taken as calendar dates (timezone-independent, DST-free).
func ScheduleWindow(schedule int, now time.Time) (bool, string) {
	if schedule <= 0 {
		schedule = 1
	}
	if now.Weekday() == DefaultScheduleDay && weekIndex(mondayOf(now))%schedule == 0 {
		return true, ""
	}
	return false, SkipReasonWindow(schedule, now)
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
