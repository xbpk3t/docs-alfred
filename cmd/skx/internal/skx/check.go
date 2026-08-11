package skx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	for k := range doc {
		if !AllowedTopLevelKeys[k] {
			return fmt.Errorf("schema %s declares unknown top-level key %q: update AllowedTopLevelKeys", schemaPath, k)
		}
	}
	if fm, ok := getMap(doc, keyFrontmatter); ok {
		for k := range fm {
			if !AllowedFrontmatterKeys[k] {
				return fmt.Errorf("schema %s declares unknown frontmatter key %q: update AllowedFrontmatterKeys", schemaPath, k)
			}
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
