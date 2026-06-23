package md

import (
	"fmt"
	"strings"
)

// --- Heading helpers ---

func h2(text string) string {
	return "## " + text
}

func h3(text string) string {
	return "### " + text
}

// --- Inline helpers ---

// Link returns a Markdown link: [text](url).
// If url is empty, returns just the text.
func Link(text, url string) string {
	if url == "" {
		return text
	}

	return fmt.Sprintf("[%s](%s)", text, url)
}

// Label returns bold inline text: **text**.
// Use for visual emphasis badges (e.g. media indicators).
func Label(text string) string {
	return fmt.Sprintf("**%s**", text)
}

// Paragraph returns a Section containing text as a plain paragraph.
func Paragraph(text string) Section {
	return &paragraphSection{text: text}
}

type paragraphSection struct {
	text string
}

func (p *paragraphSection) Markdown() string {
	return p.text
}

func (p *paragraphSection) Add(_ ...Section) {}

// blockquote wraps text as a blockquote.
func blockquote(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString("> ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}
