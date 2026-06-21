// Package md provides a component-based Markdown-to-HTML rendering pipeline.
//
// The main entry point is Document, which collects Section components
// (Table, SectionList, AIReviewItem, etc.) and renders them through
// goldmark into a complete HTML document.
//
// Usage:
//
//	doc := md.NewDocument()
//	doc.Add(md.Section("Title", md.Table(headers, rows)))
//	doc.Add(md.SectionList("Context", []string{"item1", "item2"}))
//	html, err := doc.ToHTML()
package md

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// gfm is the shared goldmark instance with GFM and common extensions.
var gfm = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Strikethrough,
		extension.TaskList,
		extension.Linkify,
		extension.Table,
		extension.DefinitionList,
		extension.Footnote,
		extension.Typographer,
		extension.NewTypographer(
			extension.WithTypographicSubstitutions(extension.TypographicSubstitutions{
				extension.LeftSingleQuote:  []byte("&sbquo;"),
				extension.RightSingleQuote: nil, // nil disables a substitution
			}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
	),
)

// ToHTML converts a full Markdown string to a complete HTML document.
// The output includes <html><head></head><body>...</body></html> wrappers.
func ToHTML(mdStr string) (string, error) {
	if mdStr == "" {
		return "", nil
	}

	var buf bytes.Buffer
	if err := gfm.Convert([]byte(mdStr), &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ToHTMLParagraph converts a Markdown string to inline HTML content
// by stripping the <html><head></head><body>...</body></html> wrapper.
// Useful for embedding content fragments in larger documents.
func ToHTMLParagraph(mdStr string) (string, error) {
	if mdStr == "" {
		return "", nil
	}

	full, err := ToHTML(mdStr)
	if err != nil {
		return "", err
	}

	// goldmark wraps output in <html><head></head><body>...</body></html>
	const bodyOpen = "<body>"
	const bodyClose = "</body>"
	start := strings.Index(full, bodyOpen)
	end := strings.LastIndex(full, bodyClose)
	if start >= 0 && end > start {
		return full[start+len(bodyOpen) : end], nil
	}

	return full, nil
}
