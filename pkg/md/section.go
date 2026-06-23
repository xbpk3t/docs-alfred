package md

import (
	"html/template"
	"strings"
)

// Section is the interface implemented by all document components.
// Each component renders itself as a Markdown string fragment.
type Section interface {
	// Markdown returns the section's content as a Markdown string.
	Markdown() string
	// Add appends child sections to this section.
	// Leaf components (Table, BulletList, etc.) implement this as a no-op.
	Add(body ...Section)
}

// section is a simple heading + body container.
type section struct {
	heading string
	body    []Section
}

// NamedSection creates a named section with optional body components.
// Renders as "## heading" followed by body content.
func NamedSection(heading string, body ...Section) Section {
	return &section{heading: heading, body: body}
}

func (s *section) Markdown() string {
	var sb strings.Builder
	sb.WriteString(h2(s.heading))
	sb.WriteString("\n\n")
	for _, b := range s.body {
		sb.WriteString(b.Markdown())
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// Add appends body sections to an existing section.
func (s *section) Add(body ...Section) {
	s.body = append(s.body, body...)
}

// Document collects Section components and renders them through goldmark.
type Document struct {
	sections []Section
}

// NewDocument creates a new empty Document.
func NewDocument() *Document {
	return &Document{}
}

// Add appends a Section to the document.
func (d *Document) Add(s Section) *Document {
	d.sections = append(d.sections, s)

	return d
}

// AddEmpty appends a plain text paragraph for empty-state messages.
func (d *Document) AddEmpty(body string) *Document {
	d.sections = append(d.sections, &emptySection{body: body})

	return d
}

type emptySection struct {
	body string
}

func (s *emptySection) Markdown() string {
	return s.body
}

func (s *emptySection) Add(_ ...Section) {}

// Markdown concatenates all sections' Markdown output.
func (d *Document) Markdown() string {
	var sb strings.Builder
	for _, s := range d.sections {
		sb.WriteString(s.Markdown())
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// ToHTML concatenates all sections' Markdown output and converts to HTML.
// The output is an HTML fragment without document-level wrappers.
func (d *Document) ToHTML() (string, error) {
	return ToHTML(d.Markdown())
}

// pageTmpl is a minimal standalone HTML page template with UTF-8 charset.
var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
{{.Content}}
</body>
</html>`))

// ToPage renders the document as a complete standalone HTML page
// with DOCTYPE, charset, and viewport meta tags.
// Use this for HTML files that will be served directly (e.g. uploaded to Litterbox).
func (d *Document) ToPage() (string, error) {
	fragment, err := d.ToHTML()
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := pageTmpl.Execute(&buf, map[string]any{
		"Content": template.HTML(fragment), //nolint:gosec // goldmark output is safe HTML
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// compile-time interface checks.
var (
	_ Section = (*section)(nil)
	_ Section = (*emptySection)(nil)
)
