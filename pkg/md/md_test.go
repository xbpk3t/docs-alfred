package md

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Atoms tests ---

func TestH2(t *testing.T) {
	assert.Equal(t, "## Title", h2("Title"))
}

func TestH3(t *testing.T) {
	assert.Equal(t, "### Sub", h3("Sub"))
}

func TestLinkWithURL(t *testing.T) {
	assert.Equal(t, "[text](https://example.com)", Link("text", "https://example.com"))
}

func TestLinkEmptyURL(t *testing.T) {
	assert.Equal(t, "just text", Link("just text", ""))
}

func TestParagraph(t *testing.T) {
	p := Paragraph("hello world")
	assert.Equal(t, "hello world", p.Markdown())
}

func TestParagraphAddIsNoOp(t *testing.T) {
	p := Paragraph("text")
	p.Add(Paragraph("child"))
	assert.Equal(t, "text", p.Markdown())
}

func TestBlockquote(t *testing.T) {
	result := blockquote("line1\nline2")
	assert.Contains(t, result, "> line1\n")
	assert.Contains(t, result, "> line2\n")
}

func TestBlockquoteTrailingNewline(t *testing.T) {
	result := blockquote("line1\nline2\n")
	assert.Contains(t, result, "> line1\n")
	assert.Contains(t, result, "> line2\n")
}

func TestBlockquoteSingleLine(t *testing.T) {
	result := blockquote("single")
	assert.Equal(t, "> single\n", result)
}

// --- Component tests ---

func TestTableMarkdown(t *testing.T) {
	tbl := Table([]string{"Name", "Value"}, [][]string{{"a", "1"}, {"b", "2"}})
	md := tbl.Markdown()
	assert.Contains(t, md, "Name")
	assert.Contains(t, md, "Value")
	assert.Contains(t, md, "a")
	assert.Contains(t, md, "1")
}

func TestTableEmpty(t *testing.T) {
	tbl := Table(nil, nil)
	// go-pretty renders an empty table as a markdown table with empty header
	tbl.Markdown() // should not panic
}

func TestTableAddIsNoOp(t *testing.T) {
	tbl := Table(nil, nil)
	before := tbl.Markdown()
	tbl.Add(Paragraph("x"))
	assert.Equal(t, before, tbl.Markdown())
}

func TestBulletListUnordered(t *testing.T) {
	l := BulletList([]string{"item1", "item2"}, false)
	md := l.Markdown()
	assert.Contains(t, md, "item1")
	assert.Contains(t, md, "item2")
}

func TestBulletListOrdered(t *testing.T) {
	l := BulletList([]string{"first", "second"}, true)
	md := l.Markdown()
	assert.Contains(t, md, "1. first")
	assert.Contains(t, md, "2. second")
}

func TestBulletListEmpty(t *testing.T) {
	l := BulletList([]string{}, false)
	l.Markdown() // should not panic
}

func TestBulletListAddIsNoOp(t *testing.T) {
	l := BulletList([]string{"x"}, false)
	before := l.Markdown()
	l.Add(Paragraph("y"))
	assert.Equal(t, before, l.Markdown())
}

func TestNoticeMarkdown(t *testing.T) {
	n := Notice("Warning", "be careful")
	md := n.Markdown()
	assert.Contains(t, md, ">")
	assert.Contains(t, md, "**Warning:**")
	assert.Contains(t, md, "be careful")
}

func TestNoticeAddIsNoOp(t *testing.T) {
	n := Notice("Info", "msg")
	before := n.Markdown()
	n.Add(Paragraph("x"))
	assert.Equal(t, before, n.Markdown())
}

func TestStatsGridMarkdown(t *testing.T) {
	stats := []StatItem{
		{Label: "Count", Value: 42},
		{Label: "Rate", Value: "95%"},
	}
	sg := StatsGrid(stats)
	md := sg.Markdown()
	assert.Contains(t, md, "Count")
	assert.Contains(t, md, "42")
	assert.Contains(t, md, "Rate")
	assert.Contains(t, md, "95%")
}

func TestStatsGridEmpty(t *testing.T) {
	sg := StatsGrid(nil)
	md := sg.Markdown()
	assert.NotEmpty(t, md)
}

func TestStatsGridAddIsNoOp(t *testing.T) {
	sg := StatsGrid(nil)
	before := sg.Markdown()
	sg.Add(Paragraph("x"))
	assert.Equal(t, before, sg.Markdown())
}

func TestMetadataMarkdown(t *testing.T) {
	m := Metadata(MdPair{Key: "Author", Value: "test"}, MdPair{Key: "Date", Value: "2024"})
	md := m.Markdown()
	assert.Contains(t, md, "- **Author:** test")
	assert.Contains(t, md, "- **Date:** 2024")
}

func TestMetadataEmpty(t *testing.T) {
	m := Metadata()
	assert.Empty(t, m.Markdown())
}

func TestMetadataAddIsNoOp(t *testing.T) {
	m := Metadata(MdPair{Key: "k", Value: "v"})
	before := m.Markdown()
	m.Add(Paragraph("x"))
	assert.Equal(t, before, m.Markdown())
}

func TestSectionListMarkdown(t *testing.T) {
	sl := SectionList("Context", []string{"item1", "item2"})
	md := sl.Markdown()
	assert.Contains(t, md, "### Context")
	assert.Contains(t, md, "item1")
	assert.Contains(t, md, "item2")
}

func TestSectionListEmpty(t *testing.T) {
	sl := SectionList("Title", []string{})
	md := sl.Markdown()
	assert.Contains(t, md, "### Title")
}

func TestSectionListAddIsNoOp(t *testing.T) {
	sl := SectionList("Title", []string{"a"})
	before := sl.Markdown()
	sl.Add(Paragraph("x"))
	assert.Equal(t, before, sl.Markdown())
}

func TestAIReviewItemMarkdown(t *testing.T) {
	sections := []ReviewSection{
		{Heading: "Progress", Items: []string{"done A", "done B"}},
		{Heading: "Next", Items: []string{"do C"}},
	}
	a := AIReviewItem(sections...)
	md := a.Markdown()
	assert.Contains(t, md, "### Progress")
	assert.Contains(t, md, "done A")
	assert.Contains(t, md, "### Next")
	assert.Contains(t, md, "do C")
}

func TestAIReviewItemSkipsEmptySections(t *testing.T) {
	sections := []ReviewSection{
		{Heading: "Empty", Items: nil},
		{Heading: "HasItems", Items: []string{"x"}},
	}
	a := AIReviewItem(sections...)
	md := a.Markdown()
	assert.NotContains(t, md, "Empty")
	assert.Contains(t, md, "HasItems")
}

func TestAIReviewItemAllEmpty(t *testing.T) {
	a := AIReviewItem(ReviewSection{Heading: "H", Items: nil})
	assert.Empty(t, a.Markdown())
}

func TestAIReviewItemAddIsNoOp(t *testing.T) {
	a := AIReviewItem(ReviewSection{Heading: "H", Items: []string{"a"}})
	before := a.Markdown()
	a.Add(Paragraph("x"))
	assert.Equal(t, before, a.Markdown())
}

// --- Section tests ---

func TestNamedSectionMarkdown(t *testing.T) {
	s := NamedSection("Intro", Paragraph("text"))
	md := s.Markdown()
	assert.Contains(t, md, "## Intro")
	assert.Contains(t, md, "text")
}

func TestNamedSectionEmptyBody(t *testing.T) {
	s := NamedSection("Title")
	md := s.Markdown()
	assert.Contains(t, md, "## Title")
}

func TestNamedSectionMultipleBody(t *testing.T) {
	s := NamedSection("Title", Paragraph("a"), Paragraph("b"))
	md := s.Markdown()
	assert.Contains(t, md, "a")
	assert.Contains(t, md, "b")
}

func TestNamedSectionAdd(t *testing.T) {
	s := NamedSection("Title", Paragraph("a"))
	s.Add(Paragraph("b"))
	md := s.Markdown()
	assert.Contains(t, md, "a")
	assert.Contains(t, md, "b")
}

// --- Document tests ---

func TestDocumentMarkdown(t *testing.T) {
	doc := NewDocument()
	doc.Add(Paragraph("first"))
	doc.Add(Paragraph("second"))
	md := doc.Markdown()
	assert.Contains(t, md, "first")
	assert.Contains(t, md, "second")
}

func TestDocumentEmpty(t *testing.T) {
	doc := NewDocument()
	assert.Empty(t, doc.Markdown())
}

func TestDocumentAddReturnsSelf(t *testing.T) {
	doc := NewDocument()
	ret := doc.Add(Paragraph("x"))
	assert.Same(t, doc, ret)
}

func TestDocumentAddEmpty(t *testing.T) {
	doc := NewDocument()
	doc.AddEmpty("nothing here")
	md := doc.Markdown()
	assert.Contains(t, md, "nothing here")
}

func TestDocumentAddEmptyReturnsSelf(t *testing.T) {
	doc := NewDocument()
	ret := doc.AddEmpty("msg")
	assert.Same(t, doc, ret)
}

func TestDocumentToHTML(t *testing.T) {
	doc := NewDocument()
	doc.Add(Paragraph("# Hello"))
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Contains(t, html, "Hello")
}

func TestDocumentToHTMLEmpty(t *testing.T) {
	doc := NewDocument()
	html, err := doc.ToHTML()
	require.NoError(t, err)
	assert.Empty(t, html)
}

func TestEmptySectionAddIsNoOp(t *testing.T) {
	s := &emptySection{body: "text"}
	before := s.Markdown()
	s.Add(Paragraph("x"))
	assert.Equal(t, before, s.Markdown())
}

// --- markdown.go tests ---

func TestToHTML(t *testing.T) {
	html, err := ToHTML("# Hello\n\nWorld")
	require.NoError(t, err)
	assert.Contains(t, html, "Hello")
	assert.Contains(t, html, "World")
}

func TestToHTMLEmpty(t *testing.T) {
	html, err := ToHTML("")
	require.NoError(t, err)
	assert.Empty(t, html)
}

func TestToHTMLParagraph(t *testing.T) {
	html, err := ToHTMLParagraph("**bold** text")
	require.NoError(t, err)
	assert.Contains(t, html, "<strong>bold</strong>")
	assert.NotContains(t, html, "<html>")
	assert.NotContains(t, html, "<body>")
}

func TestToHTMLParagraphEmpty(t *testing.T) {
	html, err := ToHTMLParagraph("")
	require.NoError(t, err)
	assert.Empty(t, html)
}

func TestToHTMLParagraphList(t *testing.T) {
	html, err := ToHTMLParagraph("- item1\n- item2")
	require.NoError(t, err)
	assert.Contains(t, html, "item1")
	assert.Contains(t, html, "item2")
}

// --- ast.go tests ---

func TestExtractTranscriptLines(t *testing.T) {
	// Note: goldmark's TableHeader is a separate kind from TableRow, so the
	// Kind check skips it. The isFirstRow flag then also skips the first data row.
	table := "| time | text |\n| --- | --- |\n| 0:00 | hello |\n| 0:05 | world |\n"
	lines := ExtractTranscriptLines(table, 1)
	require.Len(t, lines, 1)
	assert.Equal(t, "world", lines[0])
}

func TestExtractTranscriptLinesMultipleDataRows(t *testing.T) {
	table := "| a | b |\n| --- | --- |\n| skip1 | x |\n| keep2 | y |\n| keep3 | z |\n"
	lines := ExtractTranscriptLines(table, 1)
	require.Len(t, lines, 2)
	assert.Equal(t, "y", lines[0])
	assert.Equal(t, "z", lines[1])
}

func TestExtractTranscriptLinesEmptyTable(t *testing.T) {
	table := "| a | b |\n| --- | --- |\n"
	lines := ExtractTranscriptLines(table, 1)
	assert.Empty(t, lines)
}

func TestExtractTranscriptLinesNoTable(t *testing.T) {
	lines := ExtractTranscriptLines("just text", 0)
	assert.Empty(t, lines)
}

func TestExtractTranscriptLinesBilibiliFormat(t *testing.T) {
	// With 4-column table and contentCol=3, first data row is skipped.
	table := "| index | from | to | content |\n| --- | --- | --- | --- |\n| 1 | 0:00 | 0:05 | hello |\n| 2 | 0:05 | 0:10 | world |\n"
	lines := ExtractTranscriptLines(table, 3)
	require.Len(t, lines, 1)
	assert.Equal(t, "world", lines[0])
}

func TestExtractTitleFromMarkdownHeading(t *testing.T) {
	title := ExtractTitleFromMarkdown("# My Title\n\nSome content")
	assert.Equal(t, "My Title", title)
}

func TestExtractTitleFromMarkdownH2(t *testing.T) {
	title := ExtractTitleFromMarkdown("## Second Level")
	assert.Equal(t, "Second Level", title)
}

func TestExtractTitleFromMarkdownTable(t *testing.T) {
	table := "| field | value |\n| --- | --- |\n| title | Video Title |\n| author | someone |\n"
	title := ExtractTitleFromMarkdown(table)
	assert.Equal(t, "Video Title", title)
}

func TestExtractTitleFromMarkdownNoTitle(t *testing.T) {
	title := ExtractTitleFromMarkdown("just plain text")
	assert.Empty(t, title)
}

func TestExtractTitleFromMarkdownEmpty(t *testing.T) {
	title := ExtractTitleFromMarkdown("")
	assert.Empty(t, title)
}

func TestExtractTitleFromMarkdownHeadingBeatsTable(t *testing.T) {
	md := "# Heading Title\n\n| field | value |\n| --- | --- |\n| title | Table Title |\n"
	title := ExtractTitleFromMarkdown(md)
	assert.Equal(t, "Heading Title", title)
}

func TestExtractTitleFromMarkdownCaseInsensitive(t *testing.T) {
	table := "| field | value |\n| --- | --- |\n| Title | case test |\n"
	title := ExtractTitleFromMarkdown(table)
	assert.Equal(t, "case test", title)
}

// --- converter.go tests ---

func TestHTMLToMarkdown(t *testing.T) {
	md, err := HTMLToMarkdown("<h1>Hello</h1><p>World</p>")
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(md), "hello")
	assert.Contains(t, md, "World")
}

func TestHTMLToMarkdownEmpty(t *testing.T) {
	md, err := HTMLToMarkdown("")
	require.NoError(t, err)
	assert.Empty(t, md)
}

func TestHTMLToMarkdownLink(t *testing.T) {
	md, err := HTMLToMarkdown(`<a href="https://example.com">link</a>`)
	require.NoError(t, err)
	assert.Contains(t, md, "link")
	assert.Contains(t, md, "example.com")
}

func TestHTMLToMarkdownList(t *testing.T) {
	html := "<ul><li>one</li><li>two</li></ul>"
	md, err := HTMLToMarkdown(html)
	require.NoError(t, err)
	assert.Contains(t, md, "one")
	assert.Contains(t, md, "two")
}

// --- Additional coverage tests ---

func TestExtractTranscriptLinesColumnOutOfRange(t *testing.T) {
	// Column index beyond available cells - should skip that row
	table := "| a | b |\n| --- | --- |\n| x | y |\n| p | q |\n"
	lines := ExtractTranscriptLines(table, 5)
	assert.Empty(t, lines)
}

func TestExtractTitleFromTableWithNonTitleRows(t *testing.T) {
	table := "| field | value |\n| --- | --- |\n| author | someone |\n| title | Found Title |\n"
	title := ExtractTitleFromMarkdown(table)
	assert.Equal(t, "Found Title", title)
}

func TestExtractTitleFromTableNoTitleRow(t *testing.T) {
	table := "| field | value |\n| --- | --- |\n| author | someone |\n| date | 2024 |\n"
	title := ExtractTitleFromMarkdown(table)
	assert.Empty(t, title)
}

func TestExtractTitleFromTableEmptyValueCell(t *testing.T) {
	table := "| field | value |\n| --- | --- |\n| title | |\n"
	title := ExtractTitleFromMarkdown(table)
	assert.Empty(t, title)
}
