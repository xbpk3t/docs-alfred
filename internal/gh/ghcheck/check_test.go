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
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

func TestMain(m *testing.M) {
	validator.Setup()
	os.Exit(m.Run())
}

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

func TestRunCheck_ValidKindToolsNoMdscc(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "ok.yml", `- type: tunnel
  topics:
    - topic: 内网穿透工具
      kind: tools
      repo:
        - url: https://github.com/acme/frp
`)
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
	assert.Contains(t, result.Issues[0].Message, `topic "overview"`)
	assert.Contains(t, result.Issues[0].Message, "kind is required")
}

func TestRunCheck_KindUnset(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "unset.yml", `- type: tool
  topics:
    - topic: overview
      kind: unset
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, `invalid kind "unset"`)
	assert.Contains(t, result.Issues[0].Message, AllowedKindsCSV)
}

func TestRunCheck_KindFoobar(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `- type: tool
  topics:
    - topic: overview
      kind: foobar
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, `invalid kind "foobar"`)
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
	assert.Contains(t, result.Issues[0].Message, "topic is required")
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
	assert.Contains(t, result.Issues[0].Message, "type is required")
}

func TestRunCheck_RepoOnlyNoTopics(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "repo-only.yml", `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "topics is required")
}

func TestRunCheck_EmptyTopics(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "empty-topics.yml", `- type: tool
  topics: []
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "topics is required")
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
	assert.Contains(t, result.Issues[0].Message, "too many topics")
	assert.Contains(t, result.Issues[0].Message, fmt.Sprintf("%d > %d", MaxTopicsPerSection+1, MaxTopicsPerSection))
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
	assert.Contains(t, result.Issues[0].Message, "mdscc.cost is required")
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
	assert.Contains(t, result.Issues[0].Message, "mdscc.cost is required")
}

func TestRunCheck_BadYAMLContinues(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad.yml", `not: a: list: [[[`)
	writeYAML(t, dir, "sub/ok.yml", `- type: tool
  topics:
    - topic: ok
      kind: tools
`)
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

func TestRunCheck_KindRepoWithRepoFieldAndMdscc(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "repo-kind.yml", `- type: cloud-platform
  topics:
    - topic: cloudflare
      kind: repo
      `+fullMdscc+`      repo:
        - url: https://github.com/cloudflare/cloudflare-docs
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_RecursiveNested(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "infra/tunnel.yml", `- type: tunnel
  topics:
    - topic: x
      kind: howto
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}

func TestRunCheck_KindTrimmed(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "pad.yml", `- type: tool
  topics:
    - topic: overview
      kind: "  tools  "
`)
	result, err := RunCheck(dir)
	require.NoError(t, err)
	require.Empty(t, result.Issues)
}
