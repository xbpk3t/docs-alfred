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

// validFlat is a valid data/gh file under the flat-topic layout: the root is a
// list of topics (kind/topic required) with no section wrapper and no
// top-level "type" key.
const validFlat = `- topic: 内网穿透工具
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
	writeYAML(t, dir, "ok.yml", validFlat)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_ValidMech(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "ok.yml", `- topic: overview
  kind: mech
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_MdsccIsRejected(t *testing.T) {
	// mdscc was removed from the data model; the shared schema must flag it as
	// an unknown topic key.
	dir := t.TempDir()
	writeYAML(t, dir, "mdscc.yml", `- topic: overview
  kind: mech
  mdscc:
    meta: m
    derive: d
    sol: s
    cost: c
    case: k
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "additional properties 'mdscc'")
}

func TestRunCheck_MissingKind(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "missing.yml", `- topic: overview
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Equal(t, checkutil.SeverityError, result.Issues[0].Severity)
	assert.Contains(t, result.Issues[0].Message, "missing property 'kind'")
}

func TestRunCheck_KindNotInEnum(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `- topic: overview
  kind: foobar
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "kind")
}

func TestRunCheck_MissingTopicName(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "notopic.yml", `- kind: tools
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing property 'topic'")
}

func TestRunCheck_TypeImplicit(t *testing.T) {
	// The new data/gh layout derives "type" from the file name; there is no
	// top-level "type" key, so a plain topic list must validate.
	dir := t.TempDir()
	writeYAML(t, dir, "notype.yml", `- topic: overview
  kind: tools
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_RepoOnlyNoTopic(t *testing.T) {
	// A top-level item that carries only repos is not a valid topic: it must at
	// least carry topic + kind.
	dir := t.TempDir()
	writeYAML(t, dir, "repo-only.yml", `- repo:
    - url: https://github.com/acme/tool
      des: a tool
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "missing properties 'topic'")
}

func TestRunCheck_UnknownKey(t *testing.T) {
	// An undeclared topic key is an error.
	dir := t.TempDir()
	writeYAML(t, dir, "typo.yml", `- topic: overview
  kind: tools
  typoKey: 1
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "additional properties 'typoKey' not allowed")
}

func TestRunCheck_NullValue(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "null.yml", `- topic: overview
  kind: tools
  what: null
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "null")
}

func TestRunCheck_BadYAMLContinues(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `not: a: list: [[[`)
	writeYAML(t, dir, "sub/ok.yml", validFlat)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Len(t, result.Issues, 1)
	assert.Contains(t, result.Issues[0].Message, "YAML parse error")
	assert.Contains(t, result.Issues[0].File, "bad.yml")
}

func TestRunCheck_NonexistentPath(t *testing.T) {
	_, err := RunCheckWithOptions(filepath.Join(t.TempDir(), "__no_such_gh__"), CheckOptions{})
	require.Error(t, err)
}

func TestRunCheck_RecursiveNested(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "infra/tunnel.yml", validFlat)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func baselineFlat(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "- topic: t%d\n  kind: tools\n", i)
	}

	return b.String()
}

func TestRunCheck_ManyTopics(t *testing.T) {
	// The flat layout has no per-file topic cap; a file with > MaxTopicsPerSection
	// topics must still validate.
	dir := t.TempDir()
	writeYAML(t, dir, "many.yml", baselineFlat(MaxTopicsPerSection+5))
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_RelIsRepoObject(t *testing.T) {
	// rel lives on repo (not topic); each entry must be a full repo object, so a
	// bare-string rel must be rejected.
	dir := t.TempDir()
	writeYAML(t, dir, "bad-rel.yml", `- topic: x
  kind: tools
  repo:
    - url: https://github.com/acme/x
      rel:
        - https://github.com/acme/y
`)
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	require.Len(t, result.Issues, 1)
	assert.Contains(t, result.Issues[0].Message, "rel/0")
	assert.Contains(t, result.Issues[0].Message, "want object")
}
