// Package prompt loads and renders wiki AI prompt templates.
package prompt

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed prompts/*.txt
var promptFS embed.FS

// Render executes the named prompt template with data.
func Render(name string, data any) (string, error) {
	tmpl, err := template.New(name).
		Option("missingkey=error").
		ParseFS(promptFS, "prompts/"+name)
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt %s: %w", name, err)
	}
	return buf.String(), nil
}
