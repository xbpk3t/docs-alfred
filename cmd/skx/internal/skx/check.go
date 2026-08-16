package skx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/xbpk3t/docs-alfred/cmd/skx/schema"
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


// KnownNames returns the set of prompt names (frontmatter.name) present under
// dir, hidden files excluded. Used to verify pl-* dependency references exist.
func KnownNames(dir string) (map[string]bool, error) {
	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, f := range files {
		if rel, err := filepath.Rel(dir, f); err == nil && skipHidden(rel) {
			continue
		}
		if p, err := LoadPrompt(f); err == nil && p.Name != "" {
			known[p.Name] = true
		}
	}
	return known, nil
}

// CheckDir validates every *.yml under dir against the prpt JSON Schema.
// An empty schemaPath uses the schema embedded in the binary.
func CheckDir(dir, schemaPath string) (*CheckResult, error) {
	sch, err := compileSchemaOrDefault(schemaPath)
	if err != nil {
		return nil, err
	}

	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}

	known, err := KnownNames(dir)
	if err != nil {
		return nil, err
	}

	res := &CheckResult{Files: len(files)}
	for _, f := range files {
		res.Issues = append(res.Issues, checkFile(f, sch, known)...)
	}

	sort.SliceStable(res.Issues, func(i, j int) bool {
		if res.Issues[i].Path != res.Issues[j].Path {
			return res.Issues[i].Path < res.Issues[j].Path
		}
		return res.Issues[i].Message < res.Issues[j].Message
	})
	return res, nil
}

// compileSchemaOrDefault compiles the --schema path if given, else the
// embedded prpt.schema.json.
func compileSchemaOrDefault(schemaPath string) (*jsonschema.Schema, error) {
	if schemaPath != "" {
		return CompileSchema(schemaPath)
	}
	return CompileSchemaBytes(schema.Prpt)
}

// CompileSchema compiles a JSON Schema from a file path into a validator.
func CompileSchema(schemaPath string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", schemaPath, err)
	}
	return sch, nil
}

// CompileSchemaBytes compiles a JSON Schema from raw bytes (e.g. the embedded
// prpt.schema.json) into a validator.
func CompileSchemaBytes(data []byte) (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("urn:prpt", doc); err != nil {
		return nil, fmt.Errorf("add schema: %w", err)
	}
	return c.Compile("urn:prpt")
}

// CheckFile validates a single prompt file against the compiled schema.
// known is the set of existing prompt names for dependency checks; pass nil to
// skip the dangling-reference check. Callers that gate a side effect (e.g.
// render) on conformance should treat a non-empty result as "do not proceed".
func CheckFile(path string, sch *jsonschema.Schema, known map[string]bool) []Issue {
	return checkFile(path, sch, known)
}

// checkFile validates one yml against the compiled JSON Schema, plus the
// business rules that a JSON Schema cannot express (composite pipeline
// presence, dangling dependencies, folded plain scalars).
func checkFile(path string, sch *jsonschema.Schema, known map[string]bool) []Issue {
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

	// JSON Schema structural validation (unknown keys, required fields,
	// enum/type constraints) — replaces the hand-written AllowedKeys checks.
	if verr := sch.Validate(doc); verr != nil {
		add("%s", verr.Error())
	}

	checkCompositeDeps(doc, known, add)

	addFoldedScalarIssues(path, data, add)

	return issues
}

// checkCompositeDeps enforces the composite-only rules: pl-serial/pl-parallel
// must be non-empty, and every referenced dependency must exist as a prompt.
func checkCompositeDeps(doc map[string]any, known map[string]bool, add func(format string, a ...any)) {
	fm, ok := getMap(doc, keyFrontmatter)
	if !ok {
		return
	}
	role, hasRole := getString(fm, keyRole)
	if !hasRole || role != valComposite {
		return
	}

	// The pipeline section IS the composite's fan-out declaration (the schema
	// if/then requires it for composites). Every step name must resolve to an
	// existing prompt.
	parallel, serial := pipelineDeps(doc)
	deps := append(append([]string{}, serial...), parallel...)
	for _, dep := range deps {
		if known != nil && !known[dep] {
			add("pipeline dependency %q has no prompt file in references", dep)
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
