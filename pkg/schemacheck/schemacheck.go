// Package schemacheck validates YAML data files against a JSON Schema.
//
// It implements the pattern proven in cmd/skx: a JSON Schema file holds the
// structural rules (required fields, enums, unknown-key detection via
// additionalProperties), while domain-specific business rules that a JSON
// Schema cannot express (cross-file references, YAML folding quirks) are kept
// in Go and run via a PostRule hook.
package schemacheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Compile compiles a JSON Schema from a file path into a reusable validator.
func Compile(path string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", path, err)
	}

	return sch, nil
}

// CompileBytes compiles a JSON Schema from raw bytes (e.g. an embedded
// schema file) into a reusable validator.
func CompileBytes(data []byte) (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("urn:schema", doc); err != nil {
		return nil, fmt.Errorf("add schema: %w", err)
	}

	return c.Compile("urn:schema")
}

// FindSchema walks up from dir looking for a file named name, so a schema can
// live at a common ancestor of the data it describes.
func FindSchema(dir, name string) (string, error) {
	cur := dir
	for {
		candidate := filepath.Join(cur, name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("schema %s not found above %s", name, dir)
		}
		cur = parent
	}
}

// PostRule is a domain-specific hook run after schema validation. It receives
// the decoded YAML document and the file path, and returns extra issues that
// a JSON Schema cannot express. It may be nil.
type PostRule func(doc any, path string) []checkutil.Issue

// Validate validates doc against sch and returns cleaned error messages,
// one per schema violation, e.g. "/0/type: additional property 'foo' not
// allowed".
func Validate(sch *jsonschema.Schema, doc any) []string {
	if err := sch.Validate(doc); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			return formatValidationError(verr)
		}

		return []string{err.Error()}
	}

	return nil
}

// CheckFile validates a single YAML file against sch, then runs post rules.
// Empty files are skipped. Each finding becomes an error-severity Issue.
func CheckFile(path string, sch *jsonschema.Schema, post PostRule) []checkutil.Issue {
	data, err := os.ReadFile(path)
	if err != nil {
		return []checkutil.Issue{{File: path, Severity: checkutil.SeverityError, Message: fmt.Sprintf("read error: %v", err)}}
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}

	doc, err := yamlToJSON(data)
	if err != nil {
		return []checkutil.Issue{{File: path, Severity: checkutil.SeverityError, Message: fmt.Sprintf("YAML parse error: %v", err)}}
	}

	var issues []checkutil.Issue
	for _, msg := range Validate(sch, doc) {
		issues = append(issues, checkutil.Issue{File: path, Severity: checkutil.SeverityError, Message: msg})
	}
	if post != nil {
		issues = append(issues, post(doc, path)...)
	}

	return issues
}

// yamlToJSON decodes YAML into JSON-native types (map[string]any / []any /
// float64) so jsonschema/v6 can validate it without type surprises.
func yamlToJSON(data []byte) (any, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// formatValidationError extracts the leaf causes from a jsonschema/v6
// ValidationError. jsonschema/v6 renders nested errors as display lines like
// "- at '/0/topics/3': additional properties 'url' not allowed"; we keep those
// (minus the "- at " marker and path quotes) and drop the root "jsonschema
// validation failed with ..." preamble plus group nodes whose message is
// exactly "validation failed".
func formatValidationError(verr *jsonschema.ValidationError) []string {
	lines := strings.Split(strings.TrimSpace(verr.Error()), "\n")
	msgs := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "- at ") {
			ln = strings.TrimPrefix(ln, "- at ")
		} else if strings.HasPrefix(ln, "at ") {
			ln = strings.TrimPrefix(ln, "at ")
		} else {
			continue // root "jsonschema validation failed with ..." preamble
		}
		// ln is "'/path': message"
		if idx := strings.Index(ln, "': "); idx >= 0 {
			path := ln[1:idx]
			msg := ln[idx+3:]
			if msg == "validation failed" { // group node, no info on its own
				continue
			}
			msgs = append(msgs, path+": "+msg)
		}
	}
	if len(msgs) == 0 {
		msgs = append(msgs, strings.TrimSpace(verr.Error()))
	}

	return msgs
}
