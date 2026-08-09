package compact

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/pkg/mail"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// DefaultBrand is the compact product brand used for From / subject / issue title.
const DefaultBrand = "wiki compact"

// MailConfig holds Resend send parameters (token from env only).
// Brand/From comes from CompactOptions.Title via SendCompactEmail's brand arg.
type MailConfig struct {
	Token  string
	MailTo []string
}

// CompactBrand returns title or DefaultBrand.
func CompactBrand(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return DefaultBrand
	}
	return title
}

// SendCompactEmail sends HTML via Resend (thin wrapper over pkg/mail).
// brand is CompactOptions.Title (From display name); empty → DefaultBrand.
func SendCompactEmail(ctx context.Context, cfg *MailConfig, brand, subject, htmlBody string) error {
	if cfg == nil {
		return fmt.Errorf("mail config is required")
	}
	return mail.SendHTML(ctx, &mail.SendOptions{
		Token:   cfg.Token,
		To:      cfg.MailTo,
		From:    mail.DefaultFrom(CompactBrand(brand)),
		Subject: subject,
		HTML:    htmlBody,
	})
}

// CompactMailInput is data for subject/body rendering.
type CompactMailInput struct {
	Date      time.Time
	Since     time.Time
	Until     time.Time
	Title     string
	Notices   []CompactRecommend
	HotTopics []HotTopic
	// Heat/Rejected carry the gate verdicts for the transparency table.
	Heat       []HotTopic
	Rejected   []HotTopic
	Params     CompactParams
	AIFailures int
	AISkipped  bool
	SkipAI     bool
}

// CompactParams are window thresholds shown in empty/footer.
type CompactParams struct {
	SinceDuration string
	Gate          CompactGate
	BulkThreshold int
	MinDeltaChars int
	MinDeltaLines int
	TopHot        int
}

func RenderCompactSubject(in *CompactMailInput) string {
	if in == nil {
		in = &CompactMailInput{}
	}
	brand := CompactBrand(in.Title)
	day := carbon.CreateFromStdTime(in.Date).ToDateString()
	if in.AISkipped {
		if in.SkipAI {
			return fmt.Sprintf("[%s] %s — hot list (AI skipped via --skip-ai)", brand, day)
		}
		return fmt.Sprintf("[%s] %s — hot list (AI skipped)", brand, day)
	}
	n := len(in.Notices)
	if n == 0 {
		return fmt.Sprintf("[%s] %s — none", brand, day)
	}
	return fmt.Sprintf("[%s] %s — %d notices", brand, day, n)
}

// RenderCompactHTML builds the email body via pkg/md and converts to HTML.
func RenderCompactHTML(in *CompactMailInput) (string, error) {
	return buildCompactDocument(in).ToHTML()
}

// RenderCompactText is Markdown from the same document (dry-run stdout).
func RenderCompactText(in *CompactMailInput) string {
	return buildCompactDocument(in).Markdown()
}

func buildCompactDocument(in *CompactMailInput) *md.Document {
	doc := md.NewDocument()

	doc.Add(md.Paragraph(formatWindowLine(in)))

	switch {
	case in.AISkipped:
		if in.SkipAI {
			doc.Add(md.Paragraph("AI skipped intentionally (--skip-ai) — showing topics, not compact recommendations."))
		} else {
			doc.Add(md.Paragraph("AI unavailable — showing topics, not compact recommendations."))
		}
		doc.Add(heatSection(in))
	case len(in.Notices) == 0:
		doc.Add(md.Paragraph("0 compact notices in this window."))
		if len(in.Heat) > 0 {
			doc.Add(md.Paragraph(fmt.Sprintf(
				"%d topic(s) admitted; AI recommended none (or all cooled / duplicate).",
				len(in.Heat),
			)))
			doc.Add(heatSection(in))
		} else if len(in.Rejected) > 0 {
			doc.Add(md.Paragraph(fmt.Sprintf(
				"%d hot topic(s) after heat gate; AI judged none worth a blog.",
				len(in.Rejected),
			)))
			doc.Add(heatSection(in))
		} else {
			doc.Add(md.Paragraph("0 hot topics in window (heat gate rejected all)."))
		}
	default:
		for i := range in.Notices {
			if i > 0 {
				doc.Add(md.Paragraph("---"))
			}
			n := &in.Notices[i]
			var body []md.Section
			if n.SuggestedTitle != "" {
				body = append(body, md.SectionList("Title", []string{n.SuggestedTitle}))
			}
			if n.SuggestedAngle != "" {
				body = append(body, md.SectionList("Angle", []string{n.SuggestedAngle}))
			}
			if len(n.Why) > 0 {
				body = append(body, md.SectionList("Why", n.Why))
			}
			if len(n.BlogTitles) > 0 {
				body = append(body, md.SectionList("Existing blogs", n.BlogTitles))
			}
			doc.Add(md.NamedSection(n.Topic.TopicPath, body...))
		}
		doc.Add(heatSection(in))
	}

	if in.AIFailures > 0 {
		doc.Add(md.Paragraph(fmt.Sprintf(
			"AI per-topic failures: %d (treated as no)",
			in.AIFailures,
		)))
	}

	gate := in.Params.Gate
	doc.Add(md.Paragraph(fmt.Sprintf(
		"params: since=%s bulk≥%d minΔchars=%d minΔlines=%d topHot=%d gate=days≥%d,commits≥%d,Δchars≥%d",
		in.Params.SinceDuration,
		in.Params.BulkThreshold,
		in.Params.MinDeltaChars,
		in.Params.MinDeltaLines,
		in.Params.TopHot,
		gate.MinEditDays,
		gate.MinEditCommits,
		gate.MinDeltaChars,
	)))
	doc.Add(md.Paragraph("Soft reminder only — write type:blog yourself or ignore. System never auto-writes blog/log."))

	return doc
}

// heatSection renders the transparency table: every window topic with its heat
// computation, score and gate verdict. The email keeps only the admitted
// notices; `Rejected` is used when the summary-only empty state wants to show
// why nothing passed (temp partition grouped by admission for readability).
func heatSection(in *CompactMailInput) md.Section {
	headers := []string{"topic", "days", "commits", "h2", "Δchars", "score", "last", "gate", "reason"}
	rows := make([][]string, 0, len(in.Rejected))
	hots := in.Rejected
	// When we have admitted topics (they get rendered in Notices anyway), show
	// the full heat table in `Heat`; otherwise show rejected with reasons.
	switch {
	case len(in.Heat) > 0:
		hots = in.Heat
	case len(in.HotTopics) > 0:
		// AI-skip path: HotTopics carries the gate-passed list.
		hots = in.HotTopics
	}
	for i := range hots {
		h := &hots[i]
		verdict := "pass"
		reason := strings.Join(h.Reasons, "; ")
		if !h.GatePassed {
			verdict = "reject"
		}
		rows = append(rows, []string{
			h.TopicPath,
			strconv.Itoa(h.EditDays),
			strconv.Itoa(h.EditCommits),
			strconv.Itoa(h.H2Added),
			strconv.Itoa(h.DeltaChars),
			strconv.Itoa(h.Score),
			carbon.CreateFromStdTime(h.LastEdit).ToDateString(),
			verdict,
			reason,
		})
	}
	return md.NamedSection(
		fmt.Sprintf("Heat · %d (gate: days≥%d ∧ commits≥%d ∨ Δchars≥%d)", len(hots),
			in.Params.Gate.MinEditDays, in.Params.Gate.MinEditCommits, in.Params.Gate.MinDeltaChars),
		md.Table(headers, rows),
	)
}

func formatWindowLine(in *CompactMailInput) string {
	start := carbon.CreateFromStdTime(in.Since).ToDateTimeString()
	label := in.Params.SinceDuration
	if label == "" {
		label = "?"
	}
	if in.Until.IsZero() {
		return fmt.Sprintf("Window: %s since %s", label, start)
	}
	end := carbon.CreateFromStdTime(in.Until).ToDateTimeString()
	return fmt.Sprintf("Window: %s [%s, %s)", label, start, end)
}
