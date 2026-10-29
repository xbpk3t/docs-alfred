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
	keyName      = "name"
	keyRole      = "role"
	valComposite = "composite"
)

// Dispatch step sub-keys (workflow.steps items with kind: prompt).
const (
	keyKind   = "kind"
	keySteps  = "steps"
	valPrompt = "prompt"
)

// Prompt is a single parsed zzz prompt document.
type Prompt struct {
	Path string
	// Name is the routing stem derived from the file name (brk.yml → "brk"),
	// not a frontmatter field. Role is frontmatter.role.
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

	p := &Prompt{Path: path, Doc: doc, Frontmatter: fmSlice, Name: PromptStem(path)}
	if fm, ok := getMap(doc, keyFrontmatter); ok {
		if v, ok := getString(fm, keyRole); ok {
			p.Role = strings.TrimSpace(v)
		}
	}

	return p, nil
}

// PromptStem returns the routing name for a prompt file: its base file name
// without extension. Hidden files (dot-prefixed segments) are cross-ref
// targets, not routing entries, and return "".
func PromptStem(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.HasPrefix(stem, ".") {
		return ""
	}
	return stem
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

// ResolvePrompt finds the prompt whose file stem equals name under dir. It
// returns the relative references path (e.g. "analysis/3w3h") and the absolute
// path to the rendered markdown sibling. ok is false when no source .yml
// carries that stem.
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
		if _, err := LoadPrompt(f); err != nil {
			continue
		}
		return stem, f, true, nil
	}
	return "", "", false, nil
}

// AvailableNames returns all routeable prompt names under dir (sorted).
// Hidden files are skipped; names come from each file's stem.
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
		if p, err := LoadPrompt(f); err == nil && p.Name != "" {
			seen[p.Name] = true // unparseable files are surfaced by `skx check`, not here
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

// dispatchNames returns the sub-agent dispatch targets declared in the
// workflow: every steps item of shape {kind: prompt, name: <stem>}. Serial
// execution order follows workflow/phase/step order.
func dispatchNames(doc map[string]any) []string {
	var names []string
	for _, phase := range listAny(doc, keyWorkflow) {
		pm, _ := phase.(map[string]any)
		if pm == nil {
			continue
		}
		for _, s := range listAny(pm, keySteps) {
			sm, _ := s.(map[string]any)
			if sm == nil {
				continue
			}
			k, _ := sm[keyKind].(string)
			if k != valPrompt {
				continue
			}
			if n, _ := sm[keyName].(string); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// listAny returns key as a list of any (empty if absent or wrong type).
func listAny(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}
