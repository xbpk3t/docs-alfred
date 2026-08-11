package skx

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderFile(t *testing.T) {
	path := writeSample(t, t.TempDir(), "choose.yml")

	md, err := RenderFile(path)
	require.NoError(t, err)

	// Content is preserved across sections.
	for _, want := range []string{
		"name: choose",                 // frontmatter 原样
		"role: atom",                   // frontmatter 原样
		"**是什么：**",                   // what.is
		"对齐入口：先对齐再 PLAN。",        // what.is 内容
		"**不是：**",                    // what.not
		"## gate",                      // 英文 key 标题
		"| 问题 | 失败则 |",              // gate 表头
		"范围清晰吗",                     // gate qs
		"先 grill",                     // gate fail
		"### must",                     // constraint.must
		"问题必须转为选择题",               // must 内容
		"### must-not",                 // constraint.must-not
		"source: ./topic.md",           // input.source
		"### 探索",                      // workflow phase
		"**gate:** 探索完成",             // workflow gate
		"先查文件再提问",                 // workflow desc
		"检查项目配置文件",                // workflow step
		"**format:** md",               // output.format
		"**template:**",                // output.template
		"### 方案对比",                   // template 内容原样
		"**few-shot:**",                // output.few-shot
		"| # | 检查项 |",                // self-check 表头
		"是否进入 choose mode",          // self-check 内容
		"| if | then |",                // hint 表头
		"信息不足",                      // hint if
		"最多提 2-3 个选择题",             // hint then
	} {
		assert.Contains(t, md, want, "missing %q in rendered markdown", want)
	}

	// Frontmatter block is wrapped and nested (原样), not flattened.
	assert.True(t, strings.HasPrefix(md, "---\nfrontmatter:"), "frontmatter block should open the file, got:\n%s", md)

	// Section order is deterministic.
	idxWhat := strings.Index(md, "## what")
	idxGate := strings.Index(md, "## gate")
	idxHint := strings.Index(md, "## hint")
	assert.True(t, idxWhat < idxGate && idxGate < idxHint, "sections out of order")
}

func TestRenderFileOrderPreservingFrontmatter(t *testing.T) {
	path := writeSample(t, t.TempDir(), "choose.yml")
	md, err := RenderFile(path)
	require.NoError(t, err)

	// name must appear before desc (source order preserved).
	idxName := strings.Index(md, "name: choose")
	idxDesc := strings.Index(md, "desc:")
	assert.True(t, idxName >= 0 && idxName < idxDesc, "frontmatter order not preserved:\n%s", md)
}

func TestRenderFileMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/bare.yml"
	require.NoError(t, os.WriteFile(path, []byte("what:\n  is: hi\n"), 0o644))
	md, err := RenderFile(path)
	require.NoError(t, err)
	assert.NotContains(t, md, "---")
	assert.Contains(t, md, "## what")
}

func TestRenderConstraintAsList(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/conclusion.yml"
	require.NoError(t, os.WriteFile(path, []byte(`frontmatter:
  name: conclusion
  role: atom
constraint:
  - 必须按顺序输出
  - 所有内容必须在 Q&A 块中
`), 0o644))
	md, err := RenderFile(path)
	require.NoError(t, err)
	assert.Contains(t, md, "## constraint")
	assert.Contains(t, md, "1. 必须按顺序输出")
	assert.Contains(t, md, "2. 所有内容必须在 Q&A 块中")
}

func TestRenderOutputExtensionKey(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/stats.yml"
	require.NoError(t, os.WriteFile(path, []byte(`frontmatter:
  name: stats
  role: atom
output:
  format: table
  rules:
    - 保持为可见入口
    - 建议降级
`), 0o644))
	md, err := RenderFile(path)
	require.NoError(t, err)
	assert.Contains(t, md, "**format:** table")
	assert.Contains(t, md, "### rules")
	assert.Contains(t, md, "1. 保持为可见入口")
	assert.Contains(t, md, "2. 建议降级")
}
