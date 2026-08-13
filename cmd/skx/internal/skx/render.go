package skx

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// RenderFile renders a single zzz prompt yml file to markdown content.
func RenderFile(ymlPath string) (string, error) {
	p, err := LoadPrompt(ymlPath)
	if err != nil {
		return "", err
	}
	return renderPrompt(p), nil
}

// RenderResult describes one rendered file.
type RenderResult struct {
	Err     error
	YMLPath string
	MDPath  string
	Content string
}

// RenderDir renders every *.yml under dir (recursively) without writing.
func RenderDir(dir string) ([]RenderResult, error) {
	files, err := CollectYML(dir)
	if err != nil {
		return nil, err
	}
	results := make([]RenderResult, 0, len(files))
	for _, f := range files {
		r := RenderResult{YMLPath: f, MDPath: MDNameFor(f)}
		r.Content, r.Err = RenderFile(f)
		results = append(results, r)
	}
	return results, nil
}

// WriteMD writes rendered markdown content next to its source yml.
func WriteMD(ymlPath, content string) error {
	return fileutil.AtomicWriteFile(MDNameFor(ymlPath), []byte(content), 0o600)
}

// MDNameFor returns the sibling .md path for a yml file
// (references/x.yml -> references/x.md).
func MDNameFor(ymlPath string) string {
	ext := filepath.Ext(ymlPath)
	return strings.TrimSuffix(ymlPath, ext) + ".md"
}

// renderPrompt builds the markdown for a parsed prompt. Top-level keys are
// emitted in a fixed order so output is deterministic across runs.
func renderPrompt(p *Prompt) string {
	var b strings.Builder
	b.WriteString(renderFrontmatter(p))

	order := []string{keyWhat, keyGate, keyConstraint, keyInput, keyWorkflow, keyOutput, keySelfCheck, keyHint}
	for _, key := range order {
		if _, ok := p.Doc[key]; !ok {
			continue
		}
		b.WriteString("\n## " + key + "\n")
		renderSection(&b, p.Doc, key)
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderFrontmatter re-emits the frontmatter keys under a nested
// `frontmatter:` block, matching the source YAML shape and prpt.schema.json.
func renderFrontmatter(p *Prompt) string {
	if len(p.Frontmatter) == 0 {
		return ""
	}
	doc := yaml.MapSlice{{Key: keyFrontmatter, Value: p.Frontmatter}}
	raw, err := yaml.Marshal(doc)
	if err != nil {
		// Fall back to the inner slice: content preserved either way.
		raw, _ = yaml.Marshal(p.Frontmatter)
	}
	return "---\n" + string(raw) + "---\n"
}

func renderSection(b *strings.Builder, doc map[string]any, key string) {
	switch key {
	case keyWhat:
		renderWhat(b, doc)
	case keyGate:
		renderGate(b, doc)
	case keyConstraint:
		renderConstraint(b, doc)
	case keyInput:
		renderInput(b, doc)
	case keyWorkflow:
		renderWorkflow(b, doc)
	case keyOutput:
		renderOutput(b, doc)
	case keySelfCheck:
		renderSelfCheck(b, doc)
	case keyHint:
		renderHint(b, doc)
	}
}

func renderWhat(b *strings.Builder, doc map[string]any) {
	m, ok := getMap(doc, keyWhat)
	if !ok {
		return
	}
	if v, ok := getString(m, keyIs); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\n**是什么：**\n\n")
		b.WriteString(strings.TrimRight(v, "\n"))
		b.WriteString("\n")
	}
	if v, ok := getString(m, keyNot); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\n**不是：**\n\n")
		b.WriteString(strings.TrimRight(v, "\n"))
		b.WriteString("\n")
	}
}

func renderGate(b *strings.Builder, doc map[string]any) {
	renderPairTable(b, doc, keyGate, keyQS, keyFail, []string{"问题", "失败则"})
}

// renderPairTable renders a two-column table from a sequence of map items,
// each contributing one scalar under keyA and keyB.
func renderPairTable(b *strings.Builder, doc map[string]any, section, keyA, keyB string, headers []string) {
	items, ok := getSlice(doc, section)
	if !ok || len(items) == 0 {
		return
	}
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		va, _ := getString(m, keyA)
		vb, _ := getString(m, keyB)
		rows = append(rows, []string{va, vb})
	}
	b.WriteString("\n")
	b.WriteString(md.Table(headers, rows).Markdown())
}

func renderConstraint(b *strings.Builder, doc map[string]any) {
	if m, ok := getMap(doc, keyConstraint); ok {
		if must, ok := getStringSlice(m, keyMust); ok && len(must) > 0 {
			b.WriteString("\n### " + keyMust + "\n")
			for i, s := range must {
				fmt.Fprintf(b, "\n%d. %s", i+1, s)
			}
			b.WriteString("\n")
		}
		if mustNot, ok := getStringSlice(m, keyMustNot); ok && len(mustNot) > 0 {
			b.WriteString("\n### " + keyMustNot + "\n")
			for i, s := range mustNot {
				fmt.Fprintf(b, "\n%d. %s", i+1, s)
			}
			b.WriteString("\n")
		}
		return
	}

	// Some files declare constraint as a plain list of strings.
	if items, ok := getStringSlice(doc, keyConstraint); ok && len(items) > 0 {
		for i, s := range items {
			fmt.Fprintf(b, "\n%d. %s", i+1, s)
		}
		b.WriteString("\n")
	}
}

func renderInput(b *strings.Builder, doc map[string]any) {
	m, ok := getMap(doc, keyInput)
	if !ok {
		return
	}
	if v, ok := getString(m, keySource); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\nsource: " + v + "\n")
	}
	if v, ok := m[keyParams]; ok {
		b.WriteString("\nparams:\n")
		raw, _ := yaml.Marshal(v)
		b.WriteString(string(raw))
	}
}

func renderWorkflow(b *strings.Builder, doc map[string]any) {
	items, ok := getSlice(doc, keyWorkflow)
	if !ok {
		return
	}
	for _, it := range items {
		m, _ := it.(map[string]any)
		phase, _ := getString(m, keyPhase)
		if phase != "" {
			b.WriteString("\n### " + phase + "\n")
		}
		if g, ok := getString(m, keyGate); ok && strings.TrimSpace(g) != "" {
			b.WriteString("\n**gate:** " + g + "\n")
		}
		if d, ok := getString(m, keyDesc); ok && strings.TrimSpace(d) != "" {
			b.WriteString("\n" + strings.TrimRight(d, "\n") + "\n")
		}
		if steps, ok := getStringSlice(m, keySteps); ok && len(steps) > 0 {
			for i, s := range steps {
				fmt.Fprintf(b, "\n%d. %s", i+1, s)
			}
			b.WriteString("\n")
		}
	}
}

func renderOutput(b *strings.Builder, doc map[string]any) {
	m, ok := getMap(doc, keyOutput)
	if !ok {
		return
	}
	if v, ok := getString(m, keyFormat); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\n**format:** " + v + "\n")
	}
	if v, ok := m[keyStruct]; ok {
		b.WriteString("\n**struct:**\n\n")
		raw, _ := yaml.Marshal(v)
		b.WriteString(string(raw))
	}
	if v, ok := getString(m, keyTemplate); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\n**template:**\n\n")
		b.WriteString(fencedCode(v))
	}
	if v, ok := getString(m, keyFewShot); ok && strings.TrimSpace(v) != "" {
		b.WriteString("\n**few-shot:**\n\n")
		b.WriteString(fencedCode(v))
	}

	// Render every schema-valid output sub-key we have no dedicated renderer
	// for (e.g. output.rules): a list of strings becomes a numbered list,
	// anything else is re-emitted as a YAML block so no content is dropped.
	// render is gated on a passing check, so only schema-valid keys reach here.
	known := map[string]bool{keyFormat: true, keyStruct: true, keyTemplate: true, keyFewShot: true}
	var rest []string
	for k := range m {
		if !known[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		v := m[k]
		b.WriteString("\n### " + k + "\n\n")
		if list, ok := v.([]any); ok && allStrings(list) {
			for i, s := range list {
				fmt.Fprintf(b, "%d. %s\n", i+1, s)
			}
			continue
		}
		raw, _ := yaml.Marshal(v)
		b.WriteString("```yaml\n" + string(raw) + "```\n")
	}
}

// fencedCode wraps content in a fenced code block so embedded markdown
// headings (##) cannot collide with the section headings around it.
func fencedCode(v string) string {
	content := strings.TrimRight(v, "\n")
	return "```markdown\n" + content + "\n```\n"
}

func allStrings(list []any) bool {
	for _, v := range list {
		if _, ok := v.(string); !ok {
			return false
		}
	}
	return true
}

func renderSelfCheck(b *strings.Builder, doc map[string]any) {
	items, ok := getStringSlice(doc, keySelfCheck)
	if !ok || len(items) == 0 {
		return
	}
	rows := make([][]string, 0, len(items))
	for i, s := range items {
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), s})
	}
	b.WriteString("\n")
	b.WriteString(md.Table([]string{"#", "检查项"}, rows).Markdown())
}

func renderHint(b *strings.Builder, doc map[string]any) {
	renderPairTable(b, doc, keyHint, keyIf, keyThen, []string{keyIf, keyThen})
}
