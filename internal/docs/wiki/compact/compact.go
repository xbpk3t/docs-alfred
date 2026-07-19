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
// End zero means open-ended (rolling windows use End=now).
type Window struct {
	Start time.Time
	End   time.Time
	// Label is a human/params token: "last-month", "7d", "30d", …
	Label string
}

// CompactOptions controls docs-cli wiki compact.
type CompactOptions struct {
	Now              func() time.Time
	AI               *ai.ClientConfig
	Window           Window
	RepoRoot         string
	WikiRoot         string
	Mail             MailConfig
	MinDeltaChars    int
	MinDeltaLines    int
	BulkLogThreshold int
	TopNotice        int
	TopHot           int
	SendMail         bool
	DryRun           bool
	SkipAI           bool
}

// CompactResult is the pipeline outcome.
type CompactResult struct {
	Since      time.Time
	Until      time.Time
	SoftError  error
	Subject    string
	TextBody   string
	HTMLBody   string
	HotTopics  []HotTopic
	Judged     []CompactRecommend
	Notices    []CompactRecommend
	AIFailures int
	AISkipped  bool
	MailSent   bool
}

// RunCompact executes hot detect → AI → optional Resend.
func RunCompact(ctx context.Context, opts *CompactOptions) (*CompactResult, error) {
	if opts == nil {
		opts = &CompactOptions{}
	}
	normalizeCompactOpts(opts)
	now := carbon.Now().StdTime()
	if opts.Now != nil {
		now = opts.Now()
	}
	win := opts.Window
	if win.Start.IsZero() {
		win = LastMonthWindow(now)
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

	if opts.SendMail && !opts.DryRun {
		if err := SendCompactEmail(ctx, opts.Mail, result.Subject, result.HTMLBody); err != nil {
			return result, fmt.Errorf("send mail: %w", err)
		}
		result.MailSent = true
	}

	return result, nil
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

// LastMonthWindow returns the previous calendar month [start, end) in the
// configured carbon timezone (Asia/Shanghai after carboninit.Setup).
// Example: now=2026-07-15 → [2026-06-01 00:00, 2026-07-01 00:00).
func LastMonthWindow(now time.Time) Window {
	// carbon travelers mutate the receiver; Copy before SubMonth so end stays this month.
	end := carbon.CreateFromStdTime(now).StartOfMonth()
	start := end.Copy().SubMonth().StartOfMonth()
	return Window{
		Start: start.StdTime(),
		End:   end.StdTime(),
		Label: "last-month",
	}
}

// RollingWindow returns [now-d, now) with label from formatDuration.
func RollingWindow(now time.Time, d time.Duration) Window {
	if d <= 0 {
		d = 7 * 24 * time.Hour
	}
	return Window{
		Start: now.Add(-d),
		End:   now,
		Label: formatDuration(d),
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 0 && d == time.Duration(days)*24*time.Hour {
		return strconv.Itoa(days) + "d"
	}
	return d.String()
}

// ParseWindow parses a since token into a Window relative to now.
//
// Supported:
//   - "" / "last-month" / "prev-month" → previous calendar month [start, end)
//   - "7d", "30d", … → rolling [now-d, now)
//   - Go duration strings ("168h", …) → rolling
func ParseWindow(s string, now time.Time) (Window, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "last-month" || s == "prev-month" {
		return LastMonthWindow(now), nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return Window{}, fmt.Errorf("invalid since %q", s)
		}
		if n <= 0 {
			return Window{}, fmt.Errorf("invalid since %q", s)
		}
		return RollingWindow(now, time.Duration(n)*24*time.Hour), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return Window{}, fmt.Errorf("invalid since %q: %w", s, err)
	}
	if d <= 0 {
		return Window{}, fmt.Errorf("invalid since %q", s)
	}
	return RollingWindow(now, d), nil
}

// ParseSince is retained for callers that only need a rolling duration.
// Prefer ParseWindow for last-month support.
//
// Deprecated: use ParseWindow. Empty / last-month returns 0 duration (use Window).
func ParseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "last-month") || strings.EqualFold(s, "prev-month") {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid since %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
