package compact

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/audit"
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
	Title      string
	Candidates []ZeroCandidate
}

// fillMailBodies renders subject/HTML/text into result from candidates.
// The md.Document is built once and both the HTML and text views come off it.
func fillMailBodies(result *CompactResult, opts *CompactOptions, now time.Time) error {
	in := CompactMailInput{
		Date:       now,
		Title:      opts.Title,
		Candidates: result.Candidates,
	}
	result.Subject = RenderCompactSubject(&in)
	doc := buildCompactDocument(&in)
	html, err := doc.ToHTML()
	if err != nil {
		return fmt.Errorf("render compact HTML: %w", err)
	}
	result.HTMLBody = html
	result.TextBody = doc.Markdown()
	return nil
}

func RenderCompactSubject(in *CompactMailInput) string {
	if in == nil {
		in = &CompactMailInput{}
	}
	brand := CompactBrand(in.Title)
	day := carbon.CreateFromStdTime(in.Date).ToDateString()
	n := len(in.Candidates)
	if n == 0 {
		return fmt.Sprintf("[%s] %s — none", brand, day)
	}
	return fmt.Sprintf("[%s] %s — %d candidate(s)", brand, day, n)
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
	doc.Add(md.Paragraph(formatRunLine(in)))

	switch len(in.Candidates) {
	case 0:
		doc.Add(md.Paragraph("No topic cleared 清零 (empty top-N)."))
	default:
		doc.Add(md.Paragraph(fmt.Sprintf(
			"Top %d topic(s) by 清零 score (current full wiki state as this run's batch):",
			len(in.Candidates),
		)))
		doc.Add(candidateTable(in))
	}
	doc.Add(md.Paragraph("Soft reminder only — promote / clear / write type:blog yourself or ignore. System never writes blog/log.md."))
	return doc
}

// candidateTable renders the ranked 清零 candidates.
func candidateTable(in *CompactMailInput) md.Section {
	headers := []string{"rank", "topic", "files", "size", "research", "score"}
	rows := make([][]string, 0, len(in.Candidates))
	for i := range in.Candidates {
		c := &in.Candidates[i]
		rows = append(rows, []string{
			strconv.Itoa(c.Rank),
			c.Path,
			strconv.Itoa(c.Files),
			audit.HumanBytes(c.Size),
			strconv.Itoa(c.Research),
			strconv.FormatFloat(c.Score, 'f', -1, 64),
		})
	}
	return md.NamedSection("清零 · candidates", md.Table(headers, rows))
}

func formatRunLine(in *CompactMailInput) string {
	day := carbon.CreateFromStdTime(in.Date).ToDateTimeString()
	return fmt.Sprintf("清零 run %s · full wiki state as this week's batch", day)
}
