package md

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/list"
	"github.com/jedib0t/go-pretty/v6/table"
)

// --- Types ---

// StatItem is a key-value pair for stats display.
type StatItem struct {
	Value any
	Label string
}

// MdPair is a key-value metadata pair.
type MdPair struct {
	Key, Value string
}

// ReviewSection represents a single heading + items block within an AI review.
type ReviewSection struct {
	Heading string
	Items   []string
}

// --- component types (private) ---

type tbl struct {
	headers []string
	rows    [][]string
}

type bulletList struct {
	items   []string
	ordered bool
}

type notice struct {
	kind string
	msg  string
}

type statsGrid struct {
	stats []StatItem
}

type metadata struct {
	pairs []MdPair
}

type sectionList struct {
	heading string
	items   []string
}

type aiReviewItem struct {
	sections []ReviewSection
}

// --- Table ---

// Table creates a table component from headers and rows.
// Uses go-pretty table internally to render as a Markdown table.
func Table(headers []string, rows [][]string) Section {
	return &tbl{headers: headers, rows: rows}
}

func (t *tbl) Markdown() string {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)

	headerRow := make(table.Row, len(t.headers))
	for i, h := range t.headers {
		headerRow[i] = h
	}
	tw.AppendHeader(headerRow)

	for _, row := range t.rows {
		r := make(table.Row, len(row))
		for i, v := range row {
			r[i] = v
		}
		tw.AppendRow(r)
	}

	return tw.RenderMarkdown()
}

func (t *tbl) Add(_ ...Section) {}

// --- BulletList ---

// BulletList creates an ordered or unordered list.
func BulletList(items []string, ordered bool) Section {
	return &bulletList{items: items, ordered: ordered}
}

func (l *bulletList) Markdown() string {
	if l.ordered {
		var sb strings.Builder
		for i, item := range l.items {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, item)
		}

		return sb.String()
	}

	lw := list.NewWriter()
	lw.SetStyle(list.StyleDefault)
	for _, item := range l.items {
		lw.AppendItem(item)
	}

	return lw.Render()
}

func (l *bulletList) Add(_ ...Section) {}

// --- Notice ---

// Notice creates a notice/alert block using blockquote.
// kind is used as a prefix label (e.g. "Warning:", "Error:").
func Notice(kind, msg string) Section {
	return &notice{kind: kind, msg: msg}
}

func (n *notice) Markdown() string {
	return blockquote(fmt.Sprintf("**%s:** %s", n.kind, n.msg))
}

func (n *notice) Add(_ ...Section) {}

// --- StatsGrid ---

// StatsGrid renders a set of key-value stats.
// Uses go-pretty table as a simple two-column grid.
func StatsGrid(stats []StatItem) Section {
	return &statsGrid{stats: stats}
}

func (s *statsGrid) Markdown() string {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)

	tw.AppendHeader(table.Row{"", ""})
	for _, stat := range s.stats {
		tw.AppendRow(table.Row{stat.Label, fmt.Sprintf("%v", stat.Value)})
	}

	return tw.RenderMarkdown()
}

func (s *statsGrid) Add(_ ...Section) {}

// --- Metadata ---

// Metadata renders key-value pairs as inline text lines.
func Metadata(pairs ...MdPair) Section {
	return &metadata{pairs: pairs}
}

func (m *metadata) Markdown() string {
	var sb strings.Builder
	for _, p := range m.pairs {
		fmt.Fprintf(&sb, "- **%s:** %s\n", p.Key, p.Value)
	}

	return sb.String()
}

func (m *metadata) Add(_ ...Section) {}

// --- SectionList ---

// SectionList creates a heading followed by a bullet list.
// This is the basic AI content building block.
func SectionList(heading string, items []string) Section {
	return &sectionList{heading: heading, items: items}
}

func (s *sectionList) Markdown() string {
	var sb strings.Builder
	sb.WriteString(h3(s.heading))
	sb.WriteString("\n")

	lw := list.NewWriter()
	lw.SetStyle(list.StyleDefault)
	for _, item := range s.items {
		lw.AppendItem(item)
	}
	sb.WriteString(lw.Render())
	sb.WriteString("\n")

	return sb.String()
}

func (s *sectionList) Add(_ ...Section) {}

// --- AIReviewItem ---

// AIReviewItem creates a compound section from multiple heading+list blocks.
// Used for AI-generated structured reviews (e.g. progress, knowledge, review).
func AIReviewItem(sections ...ReviewSection) Section {
	return &aiReviewItem{sections: sections}
}

func (a *aiReviewItem) Markdown() string {
	var sb strings.Builder
	for _, s := range a.sections {
		if len(s.Items) == 0 {
			continue
		}
		sb.WriteString(SectionList(s.Heading, s.Items).Markdown())
	}

	return sb.String()
}

func (a *aiReviewItem) Add(_ ...Section) {}

// compile-time interface checks.
var (
	_ Section = (*tbl)(nil)
	_ Section = (*bulletList)(nil)
	_ Section = (*notice)(nil)
	_ Section = (*statsGrid)(nil)
	_ Section = (*metadata)(nil)
	_ Section = (*sectionList)(nil)
	_ Section = (*aiReviewItem)(nil)
)
