package md

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/jedib0t/go-pretty/v6/table"
)

// --- DataTable: order-preserving records, JSON as-is, Markdown as a view ---

// DataTable is a dynamic-header table over order-preserving records.
//
// Data holds one record per row; each record is a yaml.MapSlice whose key
// order is meaningful (first-seen order defines the Markdown column order).
// JSON output is the records as-is — no header synthesis, no key sorting
// (encoding/json would sort map[string]... keys; MapSlice keeps order).
// Markdown is a derived view: header = union of record keys in first-seen
// order, one column per key, missing keys render as empty cells.
//
// This mirrors the docs-images *.table.yml shape: a list of records with
// unbounded keys ("key 不限制").
type DataTable struct {
	// Data is the ordered record list; nil/empty renders an empty table.
	Data []yaml.MapSlice
}

// NewDataTable creates a DataTable from ordered records.
func NewDataTable(data []yaml.MapSlice) *DataTable {
	return &DataTable{Data: data}
}

// columns derives the dynamic header and a key → column-index map in one
// pass (first-seen order; empty when Data is empty).
func (t *DataTable) columns() ([]string, map[string]int) {
	var cols []string
	idx := make(map[string]int)
	for _, rec := range t.Data {
		for _, item := range rec {
			k := fmt.Sprintf("%v", item.Key)
			if _, seen := idx[k]; !seen {
				idx[k] = len(cols)
				cols = append(cols, k)
			}
		}
	}
	return cols, idx
}

// Columns derives the dynamic header: the union of record keys in
// first-seen order (empty when Data is empty).
func (t *DataTable) Columns() []string {
	cols, _ := t.columns()
	return cols
}

// cell returns the string value for key in the row index, or "" when absent.
func cell(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// Terminal renders Data as a terminal table (box borders, StyleLight).
func (t *DataTable) Terminal() string {
	return t.render(false)
}

// Markdown renders Data as GitHub-style Markdown table source.
func (t *DataTable) Markdown() string {
	return t.render(true)
}

// render drives go-pretty over the dynamic columns.
// markdownSource=true → RenderMarkdown, else Render (terminal box).
// Header row = Columns(); records are padded to header width with "".
func (t *DataTable) render(markdownSource bool) string {
	if len(t.Data) == 0 {
		return ""
	}
	cols, _ := t.columns()
	if len(cols) == 0 {
		return ""
	}

	tw := table.NewWriter()
	if markdownSource {
		tw.SetStyle(table.StyleDefault)
	} else {
		tw.SetStyle(table.StyleLight)
	}

	headerRow := make(table.Row, len(cols))
	for i, c := range cols {
		headerRow[i] = c
	}
	tw.AppendHeader(headerRow)

	for _, rec := range t.Data {
		row := make(map[string]any, len(rec))
		for _, item := range rec {
			row[fmt.Sprintf("%v", item.Key)] = item.Value
		}
		r := make(table.Row, len(cols))
		for i, c := range cols {
			r[i] = cell(row, c)
		}
		tw.AppendRow(r)
	}

	if markdownSource {
		return tw.RenderMarkdown()
	}
	return tw.Render()
}

// JSON emits the record list as-is (ordered keys, raw values).
// No header synthesis — Markdown is the only derived view.
//
// yaml.MapSlice has no MarshalJSON; json.Marshal would fall back to the
// struct shape {Key, Value}, so we hand-emit each record as an ordered
// JSON object (keys in MapSlice order, values converted to plain Go values).
func (t *DataTable) JSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, rec := range t.Data {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := marshalOrderedMap(&buf, rec); err != nil {
			return nil, err
		}
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// marshalOrderedMap writes one JSON object preserving MapSlice key order.
func marshalOrderedMap(buf *bytes.Buffer, rec yaml.MapSlice) error {
	buf.WriteByte('{')
	for i, item := range rec {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(fmt.Sprintf("%v", item.Key))
		if err != nil {
			return err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(item.Value)
		if err != nil {
			return err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return nil
}

// Add implements Section (leaf component, no children).
func (t *DataTable) Add(_ ...Section) {}

// --- Section view with heading ---

// NamedDataTable renders a "## heading" + dynamic table block.
// markdownSource=true → Markdown source table, else terminal box borders.
func NamedDataTable(heading string, data []yaml.MapSlice, markdownSource bool) string {
	var sb strings.Builder
	sb.WriteString(h2(heading))
	sb.WriteString("\n\n")
	dt := NewDataTable(data)
	if markdownSource {
		sb.WriteString(dt.Markdown())
	} else {
		sb.WriteString(dt.Terminal())
	}
	sb.WriteString("\n")
	return sb.String()
}

// compile-time interface checks.
var (
	_ Section = (*DataTable)(nil)
)
