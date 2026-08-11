// Package skx renders zzz prompt YAML files to markdown, validates them
// against the prpt.yml schema, and extracts their dependency graph.
//
// The render/check/graph logic lives here so every subcommand shares one
// implementation and each piece is unit-testable in isolation.
package skx

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// Document section (top-level) keys, per prpt.yml.
const (
	keyFrontmatter = "frontmatter"
	keyWhat        = "what"
	keyGate        = "gate"
	keyConstraint  = "constraint"
	keyInput       = "input"
	keyWorkflow    = "workflow"
	keyOutput      = "output"
	keySelfCheck   = "self-check"
	keyHint        = "hint"
)

// Frontmatter sub-keys.
const (
	keyName       = "name"
	keyRole       = "role"
	keyDesc       = "desc"
	keyPlSerial   = "pl-serial"
	keyPlParallel = "pl-parallel"
	keyStatus     = "status"
	keyIsSave     = "is-save"
	valComposite  = "composite"
)

// Section sub-keys.
const (
	keyIs       = "is"
	keyNot      = "not"
	keyQS       = "qs"
	keyFail     = "fail"
	keyMust     = "must"
	keyMustNot  = "must-not"
	keySource   = "source"
	keyParams   = "params"
	keyFormat   = "format"
	keyStruct   = "struct"
	keyKey      = "key"
	keyVal      = "val"
	keyTemplate = "template"
	keyFewShot  = "few-shot"
	keyRules    = "rules"
	keyPhase    = "phase"
	keySteps    = "steps"
	keyIf       = "if"
	keyThen     = "then"
)

// AllowedTopLevelKeys are the top-level document keys declared by prpt.yml.
// Unknown keys are reported by check.
var AllowedTopLevelKeys = map[string]bool{
	keyFrontmatter: true,
	keyWhat:        true,
	keyGate:        true,
	keyConstraint:  true,
	keyInput:       true,
	keyWorkflow:    true,
	keyOutput:      true,
	keySelfCheck:   true,
	keyHint:        true,
}

// AllowedFrontmatterKeys are the keys allowed inside frontmatter, per prpt.yml
// (the single source of truth). description / pipeline / pipeline-mode have
// been retired from the schema; data must use desc / pl-serial / pl-parallel.
var AllowedFrontmatterKeys = map[string]bool{
	keyName:       true,
	keyRole:       true,
	keyDesc:       true,
	keyPlSerial:   true,
	keyPlParallel: true,
	keyStatus:     true,
	keyIsSave:     true,
}

// Prompt is a single parsed zzz prompt document.
type Prompt struct {
	Path string
	// Name is frontmatter.name (the routing stem). Role is frontmatter.role.
	Name        string
	Role        string
	Doc         map[string]any
	Frontmatter yaml.MapSlice // order-preserving frontmatter, for 原样 re-emit
}

// LoadPrompt parses a zzz prompt yml file.
func LoadPrompt(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	doc, err := parseDoc(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	fmSlice, err := parseFrontmatterSlice(data)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}

	p := &Prompt{Path: path, Doc: doc, Frontmatter: fmSlice}
	if fm, ok := getMap(doc, keyFrontmatter); ok {
		if v, ok := getString(fm, keyName); ok {
			p.Name = strings.TrimSpace(v)
		}
		if v, ok := getString(fm, keyRole); ok {
			p.Role = strings.TrimSpace(v)
		}
	}

	return p, nil
}

// CollectYML returns every *.yml file under dir (recursively, sorted).
func CollectYML(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".yml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	sort.Strings(files)
	return files, nil
}

func parseDoc(data []byte) (map[string]any, error) {
	return parser.NewParser[map[string]any](data).ParseSingle()
}

// parseFrontmatterSlice decodes the frontmatter subtree into an
// order-preserving yaml.MapSlice so render can re-emit it verbatim without
// hand-writing a YAML writer.
func parseFrontmatterSlice(data []byte) (yaml.MapSlice, error) {
	var holder struct {
		Frontmatter yaml.MapSlice `yaml:"frontmatter"`
	}
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&holder); err != nil {
		return nil, err
	}
	return holder.Frontmatter, nil
}

func getMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	mm, ok := v.(map[string]any)
	return mm, ok
}

func getString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func getSlice(m map[string]any, key string) ([]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	s, ok := v.([]any)
	return s, ok
}

// getStringSlice returns key as a list of strings. A scalar string value is
// accepted as a single-element list.
func getStringSlice(m map[string]any, key string) ([]string, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	if s, isString := v.(string); isString {
		return []string{s}, true
	}
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, isString := item.(string); isString {
			out = append(out, s)
		}
	}
	return out, true
}
