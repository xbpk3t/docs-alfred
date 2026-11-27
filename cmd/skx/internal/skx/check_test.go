package skx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validBody = "## What\n\n### Is\n\nx\n\n## Output\n\n### Template\n\n```markdown\nx\n```\n"

func mdPrompt(role, desc string, pipeline []string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("role: " + role + "\n")
	if desc != "" {
		b.WriteString("desc: " + desc + "\n")
	}
	if len(pipeline) > 0 {
		b.WriteString("pipeline:\n")
		for _, p := range pipeline {
			b.WriteString("  - " + p + "\n")
		}
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	return b.String()
}

func writePrompt(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func checkDir(t *testing.T, files map[string]string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writePrompt(t, dir, name, content)
	}
	res, err := CheckDir(dir)
	require.NoError(t, err)
	return res
}

func TestCheckDirValidAtom(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, validBody)})
	assert.Equal(t, 1, res.Files)
	assert.Empty(t, res.Issues)
}

func TestCheckDirValidComposite(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("composite", "x", []string{"b"}, validBody),
		"b.md": mdPrompt("atom", "x", nil, validBody),
	})
	assert.Empty(t, res.Issues)
}

func TestCheckDirMissingFrontmatter(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": "# Title\n"})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "missing frontmatter")
	assert.True(t, res.HasErrors())
}

func TestCheckDirBadRole(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("nope", "x", nil, validBody)})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "role must be atom|composite")
}

func TestCheckDirUnknownFrontmatterKey(t *testing.T) {
	content := "---\nrole: atom\nstatus: draft\n---\n\n" + validBody
	res := checkDir(t, map[string]string{"a.md": content})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `unknown frontmatter key "status"`)
}

func TestCheckDirCompositeRequiresPipeline(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("composite", "x", nil, validBody)})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "composite requires non-empty pipeline")
}

func TestCheckDirAtomRejectsPipeline(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("atom", "x", []string{"b"}, validBody),
		"b.md": mdPrompt("atom", "x", nil, validBody),
	})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "atom must not declare pipeline")
}

func TestCheckDirDanglingPipeline(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("composite", "x", []string{"ghost"}, validBody)})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `pipeline target "ghost" has no routable prompt`)
}

func TestCheckDirSelfPipeline(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("composite", "x", []string{"a"}, validBody)})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "pipeline includes itself")
}

func TestCheckDirDuplicatePipeline(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("composite", "x", []string{"b", "b"}, validBody),
		"b.md": mdPrompt("atom", "x", nil, validBody),
	})
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, "duplicate pipeline entry")
}

func TestCheckDirH1(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, "# Title\n\n"+validBody)})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "H1 heading")
	assert.Equal(t, Error, res.Issues[0].Severity)
}

func TestCheckDirH1InsideFenceIgnored(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, "## What\n\n```text\n# comment\n```\n")})
	assert.Empty(t, res.Issues)
}

func TestCheckDirBudgetWarning(t *testing.T) {
	body := "## What\n\n" + strings.Repeat("a", BodyCharWarn+10) + "\n"
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, body)})
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, "chars >")
	assert.False(t, res.HasErrors())
}

func TestCheckDirDuplicateH2(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, "## What\n\nx\n\n## What\n\ny\n")})
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, "duplicate H2")
}

func TestCheckDirEmptyH2(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, "## What\n\n## Output\n\nx\n")})
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, "empty H2")
}

func TestCheckDirUnclosedFence(t *testing.T) {
	res := checkDir(t, map[string]string{"a.md": mdPrompt("atom", "x", nil, "## What\n\n```text\nx\n")})
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, "unclosed")
}

func TestCheckDirPipelineCycle(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("composite", "x", []string{"b"}, validBody),
		"b.md": mdPrompt("composite", "x", []string{"a"}, validBody),
	})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "pipeline cycle")
}

func TestCheckDirHiddenTargetNotRoutable(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md":       mdPrompt("composite", "x", []string{"hidden"}, validBody),
		".hidden.md": mdPrompt("atom", "x", nil, validBody),
	})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `pipeline target "hidden" has no routable prompt`)
}

func TestCheckDirHiddenFileChecked(t *testing.T) {
	res := checkDir(t, map[string]string{
		".hidden.md": mdPrompt("atom", "x", nil, "# Title\n\n"+validBody),
	})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "H1 heading")
}

func TestCheckDirH1ReportsFileLine(t *testing.T) {
	// Four frontmatter lines; the H1 is on file line 5.
	content := "---\nrole: atom\ndesc: x\n---\n# Title\n\n## A\n\nx\n"
	res := checkDir(t, map[string]string{"a.md": content})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "H1 heading at line 5")
}

func TestCheckDirIndentedCodeNotH1(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("atom", "x", nil, "## A\n\n    # not a heading\n\t# not a heading\n"),
	})
	assert.Empty(t, res.Issues)
}

func TestCheckDirIndentedFenceDoesNotHideH1(t *testing.T) {
	// Both fences are indented code blocks; the H1 at file line 7 is real.
	res := checkDir(t, map[string]string{
		"a.md": mdPrompt("atom", "x", nil, "    ```\n# real heading\n    ```\n"),
	})
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "H1 heading at line 7")
}

func TestCheckDirBOM(t *testing.T) {
	res := checkDir(t, map[string]string{
		"a.md": "\uFEFF" + mdPrompt("atom", "x", nil, validBody),
	})
	assert.Empty(t, res.Issues)
}

func TestCheckDirIndentedFrontmatterDelimiter(t *testing.T) {
	content := "---\nrole: composite\ndesc: |\n  step one\n  ---\n  step two\npipeline:\n  - b\n---\n\n## A\n\nx\n"
	res := checkDir(t, map[string]string{
		"a.md": content,
		"b.md": mdPrompt("atom", "x", nil, validBody),
	})
	assert.Empty(t, res.Issues)
}

func TestCheckDirLoadErrorPathNotDuplicated(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "a.md", "# no frontmatter\n")
	res, err := CheckDir(dir)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Equal(t, filepath.Join(dir, "a.md"), res.Issues[0].Path)
	assert.NotContains(t, res.Issues[0].Message, dir)
	assert.Contains(t, res.Issues[0].Message, "missing frontmatter")
}

func TestCheckDirDuplicateStemDoesNotLeakEdges(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "brk.md", mdPrompt("atom", "x", nil, validBody))
	writePrompt(t, dir, "x.md", mdPrompt("composite", "x", []string{"brk"}, validBody))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "zzz"), 0o755))
	writePrompt(t, dir, filepath.Join("zzz", "brk.md"), mdPrompt("composite", "x", []string{"x"}, validBody))

	res, err := CheckDir(dir)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Equal(t, filepath.Join(dir, "zzz", "brk.md"), res.Issues[0].Path)
	assert.Contains(t, res.Issues[0].Message, `duplicate routing name "brk"`)
	assert.NotContains(t, res.Issues[0].Message, "pipeline cycle")
}

func TestCheckDirShadowedSelfReference(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "brk.md", mdPrompt("atom", "x", nil, validBody))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "zzz"), 0o755))
	writePrompt(t, dir, filepath.Join("zzz", "brk.md"), mdPrompt("composite", "x", []string{"brk"}, validBody))

	res, err := CheckDir(dir)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Equal(t, Warning, res.Issues[0].Severity)
	assert.Contains(t, res.Issues[0].Message, `duplicate routing name "brk"`)
	assert.NotContains(t, res.Issues[0].Message, "pipeline includes itself")
}
