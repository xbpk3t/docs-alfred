package skx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// Issue is one validation finding for a single prompt file.
type Issue struct {
	Path    string
	Message string
}

// CheckResult aggregates validation findings across a directory.
type CheckResult struct {
	Issues []Issue
	Files  int
}

// HasErrors reports whether any issue was found.
func (r *CheckResult) HasErrors() bool { return len(r.Issues) > 0 }

// mapSectionKeys are the sub-keys allowed on map-valued sections, per prpt.yml.
var mapSectionKeys = map[string]map[string]bool{
	keyWhat:       {keyIs: true, keyNot: true},
	keyConstraint: {keyMust: true, keyMustNot: true},
	keyInput:      {keySource: true, keyParams: true},
	keyOutput:     {keyFormat: true, keyStruct: true, keyTemplate: true, keyFewShot: true, keyRules: true},
}

// seqItemKeys are the sub-keys allowed on each item of a sequence-valued
// section, per prpt.yml.
var seqItemKeys = map[string]map[string]bool{
	keyGate:     {keyQS: true, keyFail: true},
	keyWorkflow: {keyPhase: true, keyGate: true, keyDesc: true, keySteps: true},
	keyHint:     {keyIf: true, keyThen: true},
}

// FindSchema locates prpt.yml by walking up from dir. The schema is the
// source of truth sitting next to the references/ directory.
func FindSchema(dir string) (string, error) {
	cur := dir
	for {
		candidate := filepath.Join(cur, "prpt.yml")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("prpt.yml schema not found above %s", dir)
		}
		cur = parent
	}
}

// CheckDir validates every *.yml under dir against the schema.
func CheckDir(dir, schemaPath string) (*CheckResult, error) {
	if err := checkSchemaKeys(schemaPath); err != nil {
		return nil, err
	}

	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}

	res := &CheckResult{Files: len(files)}
	for _, f := range files {
		res.Issues = append(res.Issues, checkFile(f)...)
	}

	sort.SliceStable(res.Issues, func(i, j int) bool {
		if res.Issues[i].Path != res.Issues[j].Path {
			return res.Issues[i].Path < res.Issues[j].Path
		}
		return res.Issues[i].Message < res.Issues[j].Message
	})
	return res, nil
}

// checkSchemaKeys guards against schema drift: prpt.yml is the source of
// truth, so a key it declares that we do not know about is a real mismatch.
func checkSchemaKeys(schemaPath string) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	doc, err := parseDoc(data)
	if err != nil {
		return fmt.Errorf("parse schema %s: %w", schemaPath, err)
	}
	// prpt.yml now declares frontmatter keys flat (top level): name/role/desc/
	// pl-serial/pl-parallel/is-save. The section keys are still top level.
	for k := range doc {
		switch {
		case AllowedTopLevelKeys[k]:
		case AllowedFrontmatterKeys[k]:
		default:
			return fmt.Errorf("schema %s declares unknown key %q: update AllowedTopLevelKeys or AllowedFrontmatterKeys", schemaPath, k)
		}
	}
	return nil
}

// CheckFile validates a single prompt file against the schema. Callers that
// gate a side effect (e.g. render) on conformance should treat a non-empty
// result as "do not proceed".
func CheckFile(path string) []Issue {
	return checkFile(path)
}

func checkFile(path string) []Issue {
	var issues []Issue
	add := func(format string, a ...any) {
		issues = append(issues, Issue{Path: path, Message: fmt.Sprintf(format, a...)})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		add("read: %v", err)
		return issues
	}
	doc, err := parseDoc(data)
	if err != nil {
		add("parse: %v", err)
		return issues
	}

	for k := range doc {
		if !AllowedTopLevelKeys[k] {
			add("unknown top-level key %q", k)
		}
	}

	fm, ok := getMap(doc, keyFrontmatter)
	if !ok {
		add("missing required frontmatter block")
		return issues
	}
	for k := range fm {
		if !AllowedFrontmatterKeys[k] {
			add("unknown frontmatter key %q", k)
		}
	}

	name, hasName := getString(fm, keyName)
	if !hasName || strings.TrimSpace(name) == "" {
		add("missing required frontmatter.name")
	}

	role, hasRole := getString(fm, keyRole)
	if !hasRole || strings.TrimSpace(role) == "" {
		add("missing required frontmatter.role")
	}

	if role == valComposite {
		serial, _ := getStringSlice(fm, keyPlSerial)
		parallel, _ := getStringSlice(fm, keyPlParallel)
		if len(serial) == 0 && len(parallel) == 0 {
			add("role=composite requires pl-serial or pl-parallel to be non-empty")
		}
	}

	checkSectionKeys(doc, add)
	addFoldedScalarIssues(path, data, add)

	return issues
}

// checkSectionKeys reports any sub-key inside a section that is not allowed by
// the schema. Any key/value mismatch is a hard error, never silently tolerated.
func checkSectionKeys(doc map[string]any, add func(format string, a ...any)) {
	for _, sec := range []string{keyWhat, keyConstraint, keyInput, keyOutput} {
		m, ok := getMap(doc, sec)
		if !ok {
			continue
		}
		allowed := mapSectionKeys[sec]
		for k := range m {
			if !allowed[k] {
				add("unknown %s key %q", sec, k)
			}
		}
		checkStructItems(m, sec, add)
	}

	for _, sec := range []string{keyGate, keyWorkflow, keyHint} {
		items, ok := getSlice(doc, sec)
		if !ok {
			continue
		}
		allowed := seqItemKeys[sec]
		for _, it := range items {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			for k := range m {
				if !allowed[k] {
					add("unknown %s item key %q", sec, k)
				}
			}
		}
	}
}

// addFoldedScalarIssues flags multi-line plain (unquoted, unblocked) scalars:
// YAML folds their line breaks into spaces, silently destroying the content
// shape that a prompt wants to preserve. They must be written as a literal
// block (|) instead. Detected via the AST so the source style, not the
// decoded value, is inspected.
func addFoldedScalarIssues(path string, data []byte, add func(format string, a ...any)) {
	file, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return // parse errors are already reported
	}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch v := n.(type) {
		case *ast.MappingValueNode:
			if s, ok := v.Value.(*ast.StringNode); ok && s.Token != nil &&
				s.Token.Type == token.StringType && strings.Contains(s.Token.Origin, "\n") {
				add("multi-line plain scalar %q will be folded by YAML: use | block", s.Value)
			}
		case *ast.MappingNode:
			for _, kv := range v.Values {
				walk(kv.Value)
			}
		case *ast.SequenceNode:
			for _, item := range v.Values {
				walk(item)
			}
		}
	}
	if len(file.Docs) > 0 {
		walk(file.Docs[0].Body)
	}
}

// checkStructItems enforces that every output.struct entry carries both key
// and val, so the rendered field table never has a valueless column.
func checkStructItems(m map[string]any, sec string, add func(format string, a ...any)) {
	if sec != keyOutput {
		return
	}
	items, ok := m[keyStruct].([]any)
	if !ok {
		return
	}
	for _, it := range items {
		item, ok := it.(map[string]any)
		if !ok {
			continue
		}
		_, hasKey := item[keyKey]
		_, hasVal := item[keyVal]
		switch {
		case !hasKey && !hasVal:
			add("struct item missing both key and val")
		case !hasKey:
			add("struct item missing key")
		case !hasVal:
			add("struct item %q missing val", item[keyKey])
		}
	}
}
