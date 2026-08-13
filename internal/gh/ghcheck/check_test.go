package ghcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

const fullMdscc = `mdscc:
        meta: m
        derive: d
        sol: s
        cost: c
        case: k
`

const validSection = `- type: tunnel
  topics:
    - topic: 内网穿透工具
      kind: tools
      repo:
        - url: https://github.com/acme/frp
          score: 5
          des: frp
          rel:
            - url: https://github.com/acme/gofrp
              des: go 实现
      record:
        - date: 2025-08-01
          des: 初始化
          score: 5
`

func TestRunCheck_Valid(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "ok.yml", validSection)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_ValidMechWithMdscc(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "ok.yml", `- type: tool
  topics:
    - topic: overview
      kind: mech
      `+fullMdscc)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_MechWithoutMdsccOK(t *testing.T) {
	// mdscc is optional for every kind, including mech/type/repo.
	dir := t.TempDir()
	writeYAML(t, dir, "ok.yml", `- type: tool
  topics:
    - topic: overview
      kind: mech
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_MissingKind(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "missing.yml", `- type: tool
  topics:
    - topic: overview
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Equal(t, checkutil.SeverityError, result.Issues[0].Severity)
	assert.Contains(t, result.Issues[0].Message, "缺少必填字段 kind")
}

func TestRunCheck_KindNotInEnum(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `- type: tool
  topics:
    - topic: overview
      kind: foobar
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "kind")
}

func TestRunCheck_MissingTopicName(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "notopic.yml", `- type: tool
  topics:
    - kind: tools
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing property 'topic'")
}

func TestRunCheck_MissingType(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "notype.yml", `- topics:
    - topic: overview
      kind: tools
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing property 'type'")
}

func TestRunCheck_RepoOnlyNoTopics(t *testing.T) {
	// topics is required on a section (min 1), matching the old gookit rule.
	dir := t.TempDir()
	writeYAML(t, dir, "repo-only.yml", `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing property 'topics'")
}

func TestRunCheck_TooManyTopics(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("- type: tool\n  topics:\n")
	for i := 0; i < MaxTopicsPerSection+1; i++ {
		fmt.Fprintf(&b, "    - topic: t%d\n      kind: tools\n", i)
	}
	writeYAML(t, dir, "many.yml", b.String())
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "maxItems")
}

func TestRunCheck_ExactlyMaxTopics(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("- type: tool\n  topics:\n")
	for i := 0; i < MaxTopicsPerSection; i++ {
		fmt.Fprintf(&b, "    - topic: t%d\n      kind: tools\n", i)
	}
	writeYAML(t, dir, "max.yml", b.String())
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_MdsccMissingKey(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "missing-key.yml", `- type: tool
  topics:
    - topic: overview
      kind: mech
      mdscc:
        meta: m
        derive: d
        sol: s
        case: k
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing property 'cost'")
}

func TestRunCheck_MdsccEmptyField(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "empty-cost.yml", `- type: tool
  topics:
    - topic: overview
      kind: tools
      mdscc:
        meta: m
        derive: d
        sol: s
        cost: ""
        case: k
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "cost")
}

func TestRunCheck_UnknownKey(t *testing.T) {
	// The new strictness: an undeclared key is an error (was ignored before).
	dir := t.TempDir()
	writeYAML(t, dir, "typo.yml", `- type: tool
  topics:
    - topic: overview
      kind: tools
      typoKey: 1
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "additional properties 'typoKey' not allowed")
}

func TestRunCheck_NullValue(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "null.yml", `- type: tool
  topics:
    - topic: overview
      kind: tools
      what: null
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "got null")
}

func TestRunCheck_BadYAMLContinues(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `not: a: list: [[[`)
	writeYAML(t, dir, "sub/ok.yml", validSection)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Len(t, result.Issues, 1)
	assert.Contains(t, result.Issues[0].Message, "YAML parse error")
	assert.Contains(t, result.Issues[0].File, "bad.yml")
}

func TestRunCheck_NonexistentPath(t *testing.T) {
	_, err := RunCheck(filepath.Join(t.TempDir(), "__no_such_gh__"))
	require.Error(t, err)
}

func TestRunCheck_RecursiveNested(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "infra/tunnel.yml", validSection)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_RelIsRepoObject(t *testing.T) {
	// rel entries are full repo objects; a bare string rel must be rejected.
	dir := t.TempDir()
	writeYAML(t, dir, "bad-rel.yml", `- type: tool
  topics:
    - topic: x
      kind: tools
      rel:
        - https://github.com/acme/x
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
}
