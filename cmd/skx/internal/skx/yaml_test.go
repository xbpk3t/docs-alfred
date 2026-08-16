package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePrompt = `---
frontmatter:
  name: choose
  role: atom
  desc: Grill me 对齐入口；先对齐再 PLAN/写码

what:
  is: |
    对齐入口：先对齐再 PLAN。
    不要直接出 PLAN。
  not: |
    不是直接给方案
    不是模糊探索

gate:
  - qs: 范围清晰吗
    fail: 先 grill
  - qs: 有竞品吗
    fail: 跳过 vs

constraint:
  must:
    - 问题必须转为选择题
    - 最多 3 个选择题
  must-not:
    - 禁止开放性问题

input:
  source: ./topic.md
  params:
    - mode: default | heavy

workflow:
  - phase: 探索
    gate: 探索完成
    desc: 先查文件再提问
    steps:
      - 检查项目配置文件
      - 搜索现有代码
  - phase: 对齐
    steps:
      - 进入决策推荐结构

output:
  format: md
  template: |
    ### 方案对比
    | A | B |
  few-shot: |
    示例一：……

self-check:
  - 是否进入 choose mode
  - 是否给了推荐项

hint:
  - if: 信息不足
    then: 最多提 2-3 个选择题
  - if: 用户犹豫
    then: 给 PoC 方案
`

func writeSample(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(samplePrompt), 0o644))
	return path
}

func TestLoadPrompt(t *testing.T) {
	path := writeSample(t, t.TempDir(), "choose.yml")

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "choose", p.Name)
	assert.Equal(t, "atom", p.Role)
	assert.Len(t, p.Frontmatter, 3) // name, role, description
	assert.Equal(t, "name", p.Frontmatter[0].Key)
}

func TestCollectYML(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "a.yml")
	writeSample(t, dir, "b.yaml") // not collected
	writeSample(t, dir, "c.yml")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	writeSample(t, filepath.Join(dir, "sub"), "d.yml")

	files, err := CollectYML(dir)
	require.NoError(t, err)
	require.Len(t, files, 3)
	assert.Equal(t, "a.yml", filepath.Base(files[0]))
	assert.Equal(t, "d.yml", filepath.Base(files[2]))
}
