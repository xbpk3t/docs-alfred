// Package md provides a component-based Markdown-to-HTML rendering pipeline.
//
// The main entry point is Document, which collects Section components
// (Table, SectionList, AIReviewItem, etc.) and renders them through
// goldmark into HTML fragments. Use ToPage for standalone HTML documents.
//
// Usage:
//
//	doc := md.NewDocument()
//	doc.Add(md.Section("Title", md.Table(headers, rows)))
//	doc.Add(md.SectionList("Context", []string{"item1", "item2"}))
//	html, err := doc.ToPage()
package md

import (
	"bytes"

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

// ToHTML converts a Markdown string to an HTML fragment.
// The output does not include document-level wrappers.
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

// ToHTMLParagraph converts a Markdown string to inline HTML content.
// Since goldmark outputs HTML fragments directly, this function is equivalent
// to ToHTML and is kept for backward compatibility.
func ToHTMLParagraph(mdStr string) (string, error) {
	return ToHTML(mdStr)
}
