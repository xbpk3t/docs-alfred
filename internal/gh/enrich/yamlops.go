package enrich

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/yamlutil"
)

// ItemNode wraps a YAML mapping AST node representing one item in the sequence.
type ItemNode struct {
	Node    *ast.MappingNode
	pending map[string]string
	Index   int
}

// ParseYAMLFile parses a YAML file and returns all top-level sequence items.
// The returned *ast.File retains references into the parsed AST that ItemNode
// pointers point into.
func ParseYAMLFile(path string) ([]*ItemNode, *ast.File, error) {
	f, err := parser.ParseFile(path, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse yaml %s: %w", path, err)
	}

	return parseASTFile(f), f, nil
}

func parseASTFile(f *ast.File) []*ItemNode {
	var items []*ItemNode
	for _, doc := range f.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		seq, ok := doc.Body.(*ast.SequenceNode)
		if !ok {
			continue
		}
		for i, val := range seq.Values {
			mapping, ok := val.(*ast.MappingNode)
			if !ok {
				continue
			}
			items = append(items, &ItemNode{
				Node:    mapping,
				Index:   i,
				pending: make(map[string]string),
			})
		}
	}

	return items
}

// GetName returns the value of the "name" field for this item.
func (it *ItemNode) GetName() string {
	val := yamlutil.MappingValue(it.Node, "name")
	if val == nil {
		return ""
	}
	s, ok := yamlutil.String(val)
	if ok {
		return s
	}

	return ""
}

// GetPublishAt returns the value of the "publishAt" field (if any).
// Handles both string and integer node types.
func (it *ItemNode) GetPublishAt() string {
	val := yamlutil.MappingValue(it.Node, "publishAt")
	if val == nil {
		return ""
	}
	s, ok := yamlutil.String(val)
	if ok {
		return s
	}
	if intNode, ok := val.(*ast.IntegerNode); ok {
		return fmt.Sprintf("%v", intNode.Value)
	}

	return ""
}

// GetField returns the string value of a field, or "" if it doesn't exist.
// Handles both string and integer node types.
func (it *ItemNode) GetField(field string) string {
	val := yamlutil.MappingValue(it.Node, field)
	if yamlutil.IsNullOrEmptyString(val) {
		return ""
	}
	s, ok := yamlutil.String(val)
	if ok {
		return s
	}
	if intNode, ok := val.(*ast.IntegerNode); ok {
		return fmt.Sprintf("%v", intNode.Value)
	}

	return ""
}

// FieldExists reports whether the given field exists and is non-empty.
func (it *ItemNode) FieldExists(field string) bool {
	val := yamlutil.MappingValue(it.Node, field)

	return !yamlutil.IsNullOrEmptyString(val)
}

// SetField records that a field should be set. Changes are not applied until
// SaveYAMLFile is called.
func (it *ItemNode) SetField(key, value string) error {
	if key == "" || value == "" {
		return errors.New("key and value must be non-empty")
	}
	it.pending[key] = value

	return nil
}

// HasPending returns true if there are pending field changes.
func (it *ItemNode) HasPending() bool {
	return len(it.pending) > 0
}

// itemLoc holds a parsed item's location and pending field changes.
type itemLoc struct {
	mapping   *ast.MappingNode
	pending   map[string]string
	startLine int
}

// SaveYAMLFile applies all pending field changes using text-based insertion
// on the original file's source lines. This avoids a goccy/go-yaml AST bug
// where appending parsed nodes to a mapping with multi-byte characters corrupts
// the String() output.
//
// The approach:
//  1. Read the original file as lines
//  2. For each item with pending changes, find its end-of-item line
//  3. Marshal new fields as YAML, insert them after the item's last content line
//  4. Write the modified lines
func SaveYAMLFile(path string, items []*ItemNode) error {
	data, err := readFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(data, []byte("\n"))

	type insert struct {
		text      string
		afterLine int
	}
	var inserts []insert

	locs := buildItemLocations(items)

	for i, loc := range locs {
		if len(loc.pending) == 0 {
			continue
		}

		insertLine := findInsertionPoint(lines, locs, i)

		fieldLines, err := marshalPendingFields(loc.pending)
		if err != nil {
			return err
		}

		if len(fieldLines) > 0 {
			inserts = append(inserts, insert{
				afterLine: insertLine,
				text:      "\n" + strings.Join(fieldLines, "\n"),
			})
		}
	}

	// Apply inserts in reverse order (to preserve line numbers)
	sort.Slice(inserts, func(i, j int) bool {
		return inserts[i].afterLine > inserts[j].afterLine
	})

	for _, in := range inserts {
		if in.afterLine < 0 {
			in.afterLine = 0
		}

		insertLines := strings.Split(strings.TrimPrefix(in.text, "\n"), "\n")

		newLines := make([][]byte, 0, len(lines)+len(insertLines))
		newLines = append(newLines, lines[:in.afterLine+1]...)
		for _, il := range insertLines {
			newLines = append(newLines, []byte(il))
		}
		newLines = append(newLines, lines[in.afterLine+1:]...)
		lines = newLines
	}

	output := bytes.Join(lines, []byte("\n"))
	if err := fileutil.AtomicWriteFile(path, output, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write yaml %s: %w", path, err)
	}

	return nil
}

// buildItemLocations extracts item location information from ItemNode ASTs.
func buildItemLocations(items []*ItemNode) []itemLoc {
	var locs []itemLoc
	for _, item := range items {
		tok := item.Node.GetToken()
		if tok == nil {
			continue
		}
		locs = append(locs, itemLoc{
			startLine: tok.Position.Line,
			mapping:   item.Node,
			pending:   item.pending,
		})
	}
	sort.Slice(locs, func(i, j int) bool {
		return locs[i].startLine < locs[j].startLine
	})

	return locs
}

// findInsertionPoint finds the last non-empty, non-comment line within an item's range.
func findInsertionPoint(lines [][]byte, locs []itemLoc, i int) int {
	endLine := len(lines)
	if i+1 < len(locs) {
		endLine = locs[i+1].startLine - 1
	}

	insertLine := locs[i].startLine - 1
	for l := endLine - 1; l >= locs[i].startLine-1; l-- {
		if l < 0 || l >= len(lines) {
			break
		}
		trimmed := strings.TrimSpace(string(lines[l]))
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return l
		}
	}

	return insertLine
}

// marshalPendingFields converts pending field changes to indented YAML lines.
func marshalPendingFields(pending map[string]string) ([]string, error) {
	keys := make([]string, 0, len(pending))
	for k := range pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fieldLines []string
	for _, k := range keys {
		v := pending[k]
		var value any
		if listFieldValues[k] {
			value = strings.Split(v, "、")
		} else {
			value = maybeInt(v)
		}
		m := map[string]any{k: value}
		yamlBytes, err := yaml.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", k, err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(yamlBytes)), "\n") {
			fieldLines = append(fieldLines, "  "+line)
		}
	}

	return fieldLines, nil
}

// listFieldValues are YAML fields that store their values as lists (e.g., cast).
var listFieldValues = map[string]bool{
	"cast": true,
}

// readFile is a variable so it can be overridden in tests.
var readFile = os.ReadFile

// maybeInt returns s as an int if s is a pure integer string,
// or s itself otherwise. This keeps numeric YAML values unquoted.
func maybeInt(s string) any {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}

	return n
}
