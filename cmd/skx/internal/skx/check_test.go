package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSchema = `{
  "type": "object",
  "properties": {
    "frontmatter": {
      "type": "object",
      "properties": {
        "name": { "type": "string" },
        "role": { "type": "string" },
        "desc": { "type": "string" },
        "pl-serial": { "type": "array", "items": { "type": "string" } },
        "pl-parallel": { "type": "array", "items": { "type": "string" } },
        "status": { "type": "string" },
        "is-save": { "type": "boolean" }
      },
      "required": ["name", "role"],
      "additionalProperties": false
    },
    "what": { "type": "object", "properties": { "is": { "type": "string" }, "not": { "type": "string" } } },
    "gate": { "type": "array", "items": { "type": "object", "properties": { "qs": { "type": "string" }, "fail": { "type": "string" } }, "additionalProperties": false } },
    "constraint": { "type": "object", "properties": { "must": { "type": "array", "items": { "type": "string" } }, "must-not": { "type": "array", "items": { "type": "string" } } }, "additionalProperties": false },
    "input": { "type": "object", "properties": { "source": { "type": "string" }, "params": { "type": "array", "items": { "type": "object" } } }, "additionalProperties": false },
    "workflow": { "type": "array", "items": { "type": "object", "properties": { "phase": { "type": "string" }, "gate": { "type": "string" }, "desc": { "type": "string" }, "steps": { "type": "array", "items": { "type": "string" } } }, "additionalProperties": false } },
    "output": { "type": "object", "properties": { "format": { "type": "string", "enum": ["yaml", "table", "md", "artifact"] }, "struct": { "type": "array", "items": { "type": "object", "properties": { "key": { "type": "string" }, "val": { "type": ["string", "number", "boolean"] } }, "required": ["key", "val"], "additionalProperties": false } }, "template": { "type": "string" }, "few-shot": { "type": "string" } }, "additionalProperties": false },
    "self-check": { "type": "array", "items": { "type": "string" } },
    "hint": { "type": "array", "items": { "type": "object", "properties": { "if": { "type": "string" }, "then": { "type": "string" } }, "additionalProperties": false } }
  },
  "required": ["frontmatter"],
  "additionalProperties": false
}`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// setupLayout mirrors the real on-disk layout: prpt.yml sits above a
// references/ directory that holds the prompt yml files.
func setupLayout(t *testing.T, files map[string]string) (root, refs, schema string) {
	t.Helper()
	root = t.TempDir()
	refs = filepath.Join(root, "references")
	require.NoError(t, os.MkdirAll(refs, 0o755))
	schema = filepath.Join(root, "prpt.schema.json")
	writeFile(t, schema, validSchema)
	for name, content := range files {
		writeFile(t, filepath.Join(refs, name), content)
	}
	return root, refs, schema
}

func TestCheckDirValid(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": samplePrompt})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Files)
	assert.Empty(t, res.Issues)
}

func TestCheckDirMissingRequired(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  role: atom\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "name")
}

func TestCheckDirUnknownTopLevelKey(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: atom\nbogus:\n  x: 1\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "additional properties 'bogus' not allowed")
}

func TestCheckDirUnknownFrontmatterKey(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: atom\n  nope: 1\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "additional properties 'nope' not allowed")
}

func TestCheckDirCompositeRequiresPipeline(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: composite\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "pl-serial or pl-parallel")
}

func TestCheckDirCompositeWithPipelineOK(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{
		"a.yml": "frontmatter:\n  name: a\n  role: composite\n  pl-parallel:\n    - b\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Empty(t, res.Issues)
}

func TestCheckDirStrictSectionKeys(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{
		// output.format is schema-valid; a bogus sub-key must be flagged.
		"ok.yml":  "frontmatter:\n  name: ok\n  role: atom\noutput:\n  format: md\n  template: x\n",
		"bad.yml": "frontmatter:\n  name: bad\n  role: atom\noutput:\n  format: md\n  bogus: 1\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1, "only the schema-invalid sub-key should be flagged")
	assert.Contains(t, res.Issues[0].Message, "additional properties 'bogus' not allowed")

	if !assert.True(t, res.HasErrors()) {
		return
	}
}

func TestCheckDirStrictWorkflowItemKeys(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{
		"a.yml": "frontmatter:\n  name: a\n  role: atom\nworkflow:\n  - phase: x\n    bloop: 1\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "additional properties 'bloop' not allowed")
}

func TestCheckSchemaDrift(t *testing.T) {
	dir := t.TempDir()
	// Not a valid JSON Schema → compile fails, surfaced as an error.
	writeFile(t, filepath.Join(dir, "prpt.schema.json"), "brand-new: 1\n")

	_, err := CheckDir(dir, filepath.Join(dir, "prpt.schema.json"))
	require.Error(t, err)
}

func TestCheckDirNestedSchemaOK(t *testing.T) {
	// The prpt JSON Schema uses a nested frontmatter: object (the intended form).
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, os.MkdirAll(refs, 0o755))
	writeFile(t, filepath.Join(root, "prpt.schema.json"), validSchema)
	writeFile(t, filepath.Join(refs, "a.yml"), samplePrompt)

	res, err := CheckDir(refs, filepath.Join(root, "prpt.schema.json"))
	require.NoError(t, err)
	assert.Empty(t, res.Issues)
}

func TestFindSchema(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "prpt.schema.json")
	writeFile(t, schema, validSchema)

	got, err := FindSchema(filepath.Join(dir, "references", "analysis"))
	require.NoError(t, err)
	assert.Equal(t, schema, got)
}
