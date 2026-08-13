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
	"github.com/santhosh-tekuri/jsonschema/v6"
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


// FindSchema locates the prpt JSON Schema by walking up from dir. It prefers
// prpt.schema.json (the JSON Schema used for validation) and falls back to
// prpt.yml (the documented schema) so both layouts work.
func FindSchema(dir string) (string, error) {
	cur := dir
	for {
		for _, name := range []string{"prpt.schema.json", "prpt.yml"} {
			candidate := filepath.Join(cur, name)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return candidate, nil
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("prpt schema (prpt.schema.json) not found above %s", dir)
		}
		cur = parent
	}
}

// CheckDir validates every *.yml under dir against the prpt JSON Schema.
func CheckDir(dir, schemaPath string) (*CheckResult, error) {
	sch, err := CompileSchema(schemaPath)
	if err != nil {
		return nil, err
	}

	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}

	res := &CheckResult{Files: len(files)}
	for _, f := range files {
		res.Issues = append(res.Issues, checkFile(f, sch)...)
	}

	sort.SliceStable(res.Issues, func(i, j int) bool {
		if res.Issues[i].Path != res.Issues[j].Path {
			return res.Issues[i].Path < res.Issues[j].Path
		}
		return res.Issues[i].Message < res.Issues[j].Message
	})
	return res, nil
}

// CompileSchema compiles the prpt.schema.json JSON Schema into a validator.
func CompileSchema(schemaPath string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", schemaPath, err)
	}
	return sch, nil
}

// CheckFile validates a single prompt file against the compiled schema.
// Callers that gate a side effect (e.g. render) on conformance should treat a
// non-empty result as "do not proceed".
func CheckFile(path string, sch *jsonschema.Schema) []Issue {
	return checkFile(path, sch)
}

// checkFile validates one yml against the compiled JSON Schema, plus the
// business rules that a JSON Schema cannot express (composite pipeline
// presence, folded plain scalars).
func checkFile(path string, sch *jsonschema.Schema) []Issue {
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

	if fm, ok := getMap(doc, keyFrontmatter); ok {
		if role, hasRole := getString(fm, keyRole); hasRole && role == valComposite {
			serial, _ := getStringSlice(fm, keyPlSerial)
			parallel, _ := getStringSlice(fm, keyPlParallel)
			if len(serial) == 0 && len(parallel) == 0 {
				add("role=composite requires pl-serial or pl-parallel to be non-empty")
			}
		}
	}

	addFoldedScalarIssues(path, data, add)

	return issues
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
