package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleBody = `## What

### Is
对齐入口：先对齐再 PLAN。

### Is not
不是直接给方案。

## Output

### Template

` + "```markdown" + `
### Comparison
| A | B |
` + "```" + `
`

var samplePrompt = mdPrompt("atom", "Grill me 对齐入口", nil, sampleBody)

func writeSample(t *testing.T, dir, name string) string {
	t.Helper()
	return writePrompt(t, dir, name, samplePrompt)
}

func TestLoadPrompt(t *testing.T) {
	dir := t.TempDir()
	path := writeSample(t, dir, "choose.md")

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "choose", p.Name)
	assert.Equal(t, "atom", p.Role)
	assert.Equal(t, "Grill me 对齐入口", p.Desc)
	assert.Contains(t, p.Body, "## Output")
	info := analyzeMarkdown(p.Body)
	assert.Empty(t, info.H1)
	assert.Empty(t, info.DuplicateH2)
}

func TestLoadPromptCompositePipeline(t *testing.T) {
	dir := t.TempDir()
	content := "---\nrole: composite\ndesc: x\npipeline:\n  - a\n  - b\n---\n\n## Workflow\n\n### Step\n\n1. x\n"
	path := filepath.Join(dir, "comp.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "comp", p.Name)
	assert.Equal(t, "composite", p.Role)
	assert.Equal(t, []string{"a", "b"}, p.Pipeline)
}

func TestCollectMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "a.md")
	writeSample(t, dir, "b.yaml") // not collected
	writeSample(t, dir, "c.md")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	writeSample(t, filepath.Join(dir, "sub"), "d.md")
	writeSample(t, dir, ".hidden.md")

	files, err := CollectMarkdown(dir)
	require.NoError(t, err)
	require.Len(t, files, 4)
	assert.Equal(t, ".hidden.md", filepath.Base(files[0]))
	assert.Equal(t, "d.md", filepath.Base(files[3]))
}

func TestResolvePromptSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, ".hidden.md")
	writeSample(t, dir, "visible.md")

	_, _, ok, err := ResolvePrompt(dir, "hidden")
	require.NoError(t, err)
	assert.False(t, ok)

	rel, abs, ok, err := ResolvePrompt(dir, "visible")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "visible", rel)
	assert.Equal(t, filepath.Join(dir, "visible.md"), abs)
}

func TestAvailableNamesSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, ".hidden.md")
	writeSample(t, dir, "visible.md")

	names, err := AvailableNames(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"visible"}, names)
}

func TestLoadPromptBOM(t *testing.T) {
	dir := t.TempDir()
	content := "\uFEFF" + mdPrompt("atom", "x", nil, validBody)
	path := writePrompt(t, dir, "bom.md", content)

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "atom", p.Role)
	assert.Equal(t, 4, p.FrontmatterLines)
}

func TestLoadPromptIndentedDelimiterInsideBlockScalar(t *testing.T) {
	dir := t.TempDir()
	content := "---\nrole: composite\ndesc: |\n  step one\n  ---\n  step two\npipeline:\n  - b\n---\n\n## A\n"
	path := writePrompt(t, dir, "a.md", content)

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "composite", p.Role)
	assert.Equal(t, "step one\n---\nstep two", p.Desc)
	assert.Equal(t, []string{"b"}, p.Pipeline)
	assert.Equal(t, 9, p.FrontmatterLines)
	assert.NotContains(t, p.Body, "pipeline:")
}

func TestSplitFrontmatterRequiresColumnZero(t *testing.T) {
	_, _, _, err := splitFrontmatter([]byte("  ---\nrole: atom\n---\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing frontmatter")

	// An indented line does not close frontmatter, so this file is unclosed.
	_, _, _, err = splitFrontmatter([]byte("---\nrole: atom\n  ---\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed frontmatter")
}

func TestAnalyzeMarkdownIgnoresIndentedCode(t *testing.T) {
	info := analyzeMarkdown("## A\n\n    # four spaces\n\t# tab\n")
	assert.Empty(t, info.H1)
}

func TestAnalyzeMarkdownIgnoresIndentedFence(t *testing.T) {
	// Four-space-indented backticks are an indented code block, not a fence,
	// so the H1 between them must still be detected.
	info := analyzeMarkdown("    ```\n# real heading\n    ```\n")
	require.Len(t, info.H1, 1)
	assert.Equal(t, 2, info.H1[0].Line)
	assert.Empty(t, info.FenceIssues)
}

func TestAnalyzeMarkdownFromOffset(t *testing.T) {
	info := analyzeMarkdownFrom("## A\n\n# B\n", 4)
	require.Len(t, info.H1, 1)
	assert.Equal(t, 7, info.H1[0].Line)

	info = analyzeMarkdownFrom("```text\nx\n", 4)
	require.Len(t, info.FenceIssues, 1)
	assert.Contains(t, info.FenceIssues[0], "line 5")
}
