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
	keyPipeline    = "pipeline"
	keyWorkflow    = "workflow"
	keyOutput      = "output"
	keySelfCheck   = "self-check"
	keyHint        = "hint"
)

// Frontmatter sub-keys.
const (
	keyName      = "name"
	keyRole      = "role"
	valComposite = "composite"
)

// Pipeline sub-keys (composite orchestration, see prpt.schema.json).
const (
	keyParallel = "parallel"
	keySerial   = "serial"
)

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

// skipHidden reports whether a relative path contains a dot-segment
// (e.g. repo/.TableCate.yml): hidden files are cross-ref targets, not
// routing entries.
func skipHidden(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(seg, ".") && seg != "." && seg != ".." {
			return true
		}
	}
	return false
}

// ResolvePrompt finds the prompt whose frontmatter.name equals name under
// dir. It returns the relative references path (e.g. "analysis/3w3h") and the
// absolute path to the rendered markdown sibling. ok is false when no source
// .yml carries that name.
func ResolvePrompt(dir, name string) (rel, ymlAbs string, ok bool, err error) {
	files, err := CollectYML(dir)
	if err != nil {
		return "", "", false, err
	}
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil || skipHidden(rel) {
			continue
		}
		stem := strings.TrimSuffix(rel, filepath.Ext(rel))
		if filepath.Base(stem) != name {
			continue
		}
		p, err := LoadPrompt(f)
		if err != nil {
			continue
		}
		if p.Name == name {
			return stem, f, true, nil
		}
	}
	return "", "", false, nil
}

// AvailableNames returns all routeable prompt names under dir (sorted).
// Hidden files are skipped; names come straight from the source yml files.
func AvailableNames(dir string) ([]string, error) {
	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil || skipHidden(rel) {
			continue
		}
		p, err := LoadPrompt(f)
		if err != nil {
			continue // unparseable files are surfaced by `skx check`, not here
		}
		if p.Name != "" {
			seen[p.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
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

// pipelineDeps returns the fan-out step names declared in the pipeline
// section: parallel (pipeline.parallel) and serial (pipeline.serial). Each
// step is an object whose name is the target prompt stem.
func pipelineDeps(doc map[string]any) (parallel, serial []string) {
	pipe, ok := getMap(doc, keyPipeline)
	if !ok {
		return nil, nil
	}
	for _, v := range listAny(pipe, keyParallel) {
		if n := stepName(v); n != "" {
			parallel = append(parallel, n)
		}
	}
	for _, v := range listAny(pipe, keySerial) {
		if n := stepName(v); n != "" {
			serial = append(serial, n)
		}
	}
	return parallel, serial
}

// listAny returns key as a list of any (empty if absent or wrong type).
func listAny(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

// stepName extracts a pipeline step object's name field.
func stepName(v any) string {
	m, _ := v.(map[string]any)
	if m == nil {
		return ""
	}
	s, _ := m[keyName].(string)
	return s
}
