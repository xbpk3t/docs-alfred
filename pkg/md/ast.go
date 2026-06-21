package md

import (
	"strings"

	gast "github.com/yuin/goldmark/ast"
	geast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ExtractTranscriptLines parses a markdown table using goldmark and returns
// the text from the specified column index (0-based). Skips table header rows.
// Used for both bilibili subtitle (| index | from | to | content |, col=3)
// and YouTube transcript (| timestamp | speaker | text |, col=2) tables.
func ExtractTranscriptLines(mdBody string, contentCol int) []string {
	reader := text.NewReader([]byte(mdBody))
	doc := gfm.Parser().Parse(reader)

	var lines []string
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if n.Kind() != geast.KindTable {
			return gast.WalkContinue, nil
		}
		lines = extractTableLinesFromRow(n, mdBody, contentCol)

		return gast.WalkStop, nil
	})

	return lines
}

// extractTableLinesFromRow iterates over table rows and extracts text from the specified column.
// It skips the header row and returns non-empty cell text.
func extractTableLinesFromRow(n gast.Node, mdBody string, contentCol int) []string {
	var lines []string
	isFirstRow := true
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Kind() != geast.KindTableRow {
			continue
		}
		// Skip the header row (first row after the table node).
		if isFirstRow {
			isFirstRow = false

			continue
		}
		cell := row.FirstChild()
		for i := 0; i < contentCol && cell != nil; i++ {
			cell = cell.NextSibling()
		}
		if cell == nil || cell.Kind() != geast.KindTableCell {
			continue
		}
		cellText := strings.TrimSpace(string(cell.Text([]byte(mdBody))))
		if cellText == "" {
			continue
		}
		lines = append(lines, cellText)
	}

	return lines
}

// ExtractTitleFromMarkdown parses the markdown body with goldmark and returns
// the title from either:
//   - the first heading (any level), or
//   - a metadata-style table (| field | value |) with a "title" field.
//
// This handles all opencli adapters — those that return heading-prefixed
// markdown (twitter, web read, etc.) and those that return metadata tables
// (bilibili video, etc.).
func ExtractTitleFromMarkdown(body string) string {
	reader := text.NewReader([]byte(body))
	doc := gfm.Parser().Parse(reader)

	var title string
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}

		// First heading (any level) wins.
		if n.Kind() == gast.KindHeading {
			title = strings.TrimSpace(string(n.Text([]byte(body))))

			return gast.WalkStop, nil
		}

		// Table with | field | value | structure — look for a "title" row.
		if n.Kind() == geast.KindTable {
			if t := extractTitleFromTable(body, n); t != "" {
				title = t

				return gast.WalkStop, nil
			}
		}

		return gast.WalkContinue, nil
	})

	return title
}

// extractTitleFromTable walks a goldmark Table AST node looking for a row
// whose first cell is "title" and returns the value from the second cell.
func extractTitleFromTable(body string, table gast.Node) string {
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Kind() != geast.KindTableRow {
			continue
		}
		cell := row.FirstChild()
		if cell == nil || cell.Kind() != geast.KindTableCell {
			continue
		}
		field := strings.TrimSpace(string(cell.Text([]byte(body))))
		if !strings.EqualFold(field, "title") {
			continue
		}
		valueCell := cell.NextSibling()
		if valueCell == nil || valueCell.Kind() != geast.KindTableCell {
			continue
		}

		return strings.TrimSpace(string(valueCell.Text([]byte(body))))
	}

	return ""
}
