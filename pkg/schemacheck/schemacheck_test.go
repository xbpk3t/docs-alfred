package schemacheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

const testSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "array",
  "items": {
    "type": "object",
    "required": ["name"],
    "additionalProperties": false,
    "properties": {
      "name": { "type": "string", "minLength": 1 },
      "kind": { "enum": ["mech", "type"] }
    }
  }
}`

func compileTestSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	sch, err := CompileBytes([]byte(testSchema))
	require.NoError(t, err)

	return sch
}

func TestCompileBytes_InvalidJSON(t *testing.T) {
	_, err := CompileBytes([]byte("{not json"))
	require.Error(t, err)
}

func TestCompileBytes_InvalidSchema(t *testing.T) {
	_, err := CompileBytes([]byte(`{"type": "not-a-real-type"}`))
	require.Error(t, err)
}

func TestCompile_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.schema.json")
	require.NoError(t, os.WriteFile(path, []byte(testSchema), 0o644))

	sch, err := Compile(path)
	require.NoError(t, err)
	require.NotNil(t, sch)

	_, err = Compile(filepath.Join(dir, "missing.schema.json"))
	require.Error(t, err)
}

func TestFindSchema_WalksUp(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.schema.json"), []byte("{}"), 0o644))
	nested := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	found, err := FindSchema(nested, "x.schema.json")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "x.schema.json"), found)

	_, err = FindSchema(nested, "nope.schema.json")
	require.Error(t, err)
}

func TestValidate_ValidDoc(t *testing.T) {
	sch := compileTestSchema(t)
	msgs := Validate(sch, []any{map[string]any{"name": "a", "kind": "mech"}})
	assert.Empty(t, msgs)
}

func TestValidate_InvalidDoc(t *testing.T) {
	sch := compileTestSchema(t)
	msgs := Validate(sch, []any{map[string]any{"name": ""}})
	require.NotEmpty(t, msgs)
	// leaf cause formatted as "/N/name: message", no "jsonschema validation failed" preamble
	assert.NotContains(t, msgs[0], "jsonschema validation failed")
	assert.NotContains(t, msgs[0], "at '/")
	assert.Contains(t, msgs[0], "/0/name:")
}

func TestValidate_UnknownKey(t *testing.T) {
	sch := compileTestSchema(t)
	msgs := Validate(sch, []any{map[string]any{"name": "a", "typo": "x"}})
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "additional properties 'typo' not allowed")
}

func TestCheckFile_Valid(t *testing.T) {
	sch := compileTestSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.yml")
	require.NoError(t, os.WriteFile(path, []byte("- name: a\n  kind: type\n"), 0o644))

	issues := CheckFile(path, sch, nil)
	assert.Empty(t, issues)
}

func TestCheckFile_Invalid(t *testing.T) {
	sch := compileTestSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte("- name: a\n  nope: x\n"), 0o644))

	issues := CheckFile(path, sch, nil)
	require.NotEmpty(t, issues)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
	assert.Equal(t, path, issues[0].File)
}

func TestCheckFile_EmptyFile(t *testing.T) {
	sch := compileTestSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yml")
	require.NoError(t, os.WriteFile(path, []byte("   \n\n"), 0o644))

	issues := CheckFile(path, sch, nil)
	assert.Empty(t, issues)
}

func TestCheckFile_ParseError(t *testing.T) {
	sch := compileTestSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte("not: a: list: [[[ "), 0o644))

	issues := CheckFile(path, sch, nil)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "YAML parse error")
}

func TestCheckFile_Unreadable(t *testing.T) {
	sch := compileTestSchema(t)
	issues := CheckFile(filepath.Join(t.TempDir(), "missing.yml"), sch, nil)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "read error")
}

func TestCheckFile_PostRule(t *testing.T) {
	sch := compileTestSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.yml")
	require.NoError(t, os.WriteFile(path, []byte("- name: a\n"), 0o644))

	post := func(doc any, p string) []checkutil.Issue {
		return []checkutil.Issue{{File: p, Severity: checkutil.SeverityError, Message: "business rule"}}
	}
	issues := CheckFile(path, sch, post)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "business rule")
}
