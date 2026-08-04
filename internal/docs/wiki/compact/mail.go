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
	Date       time.Time
	Since      time.Time
	Until      time.Time
	Title      string
	Notices    []CompactRecommend
	HotTopics  []HotTopic
	Params     CompactParams
	AIFailures int
	AISkipped  bool
	SkipAI     bool
}

// CompactParams are window thresholds shown in empty/footer.
type CompactParams struct {
	SinceDuration string
	BulkThreshold int
	MinDeltaChars int
	MinDeltaLines int
	TopHot        int
	TopNotice     int
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
			doc.Add(md.Paragraph("AI skipped intentionally (--skip-ai) — showing hot topics only (not compact recommendations)."))
		} else {
			doc.Add(md.Paragraph("AI unavailable — showing hot topics only (not compact recommendations)."))
		}
		if len(in.HotTopics) > 0 {
			doc.Add(hotTopicsSection(in.HotTopics))
		}
	case len(in.Notices) == 0:
		doc.Add(md.Paragraph("0 compact notices in this window."))
		if len(in.HotTopics) > 0 {
			doc.Add(md.Paragraph(fmt.Sprintf(
				"%d hot topic(s) after filters; AI recommended none (or all cooled).",
				len(in.HotTopics),
			)))
			doc.Add(hotTopicsSection(in.HotTopics))
		} else {
			doc.Add(md.Paragraph("0 hot topics in window."))
		}
	default:
		for i := range in.Notices {
			if i > 0 {
				doc.Add(md.Paragraph("---"))
			}
			n := &in.Notices[i]
			var body []md.Section
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
	}

	if in.AIFailures > 0 {
		doc.Add(md.Paragraph(fmt.Sprintf(
			"AI per-topic failures: %d (treated as no)",
			in.AIFailures,
		)))
	}

	doc.Add(md.Paragraph(fmt.Sprintf(
		"params: since=%s bulk≥%d minΔchars=%d minΔlines=%d topHot=%d topNotice=%d",
		in.Params.SinceDuration,
		in.Params.BulkThreshold,
		in.Params.MinDeltaChars,
		in.Params.MinDeltaLines,
		in.Params.TopHot,
		in.Params.TopNotice,
	)))
	doc.Add(md.Paragraph("Soft reminder only — write type:blog yourself or ignore. System never auto-writes blog/log."))

	return doc
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

func hotTopicsSection(hots []HotTopic) md.Section {
	headers := []string{"topic", "days", "commits", "Δchars", "score", "last"}
	rows := make([][]string, 0, len(hots))
	for i := range hots {
		h := &hots[i]
		rows = append(rows, []string{
			h.TopicPath,
			strconv.Itoa(h.EditDays),
			strconv.Itoa(h.EditCommits),
			strconv.Itoa(h.DeltaChars),
			strconv.Itoa(h.Score),
			carbon.CreateFromStdTime(h.LastEdit).ToDateString(),
		})
	}
	return md.NamedSection(
		fmt.Sprintf("Hot topics · %d", len(hots)),
		md.Table(headers, rows),
	)
}
