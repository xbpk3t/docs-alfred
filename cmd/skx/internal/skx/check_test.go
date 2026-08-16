package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	prptschema "github.com/xbpk3t/docs-alfred/cmd/skx/schema"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// setupLayout mirrors the real on-disk layout: prpt.yml sits above a
// references/ directory that holds the prompt yml files. The schema used is
// the real embedded prpt.schema.json (schema.Prpt) so tests never drift from
// the source of truth.
func setupLayout(t *testing.T, files map[string]string) (root, refs, schema string) {
	t.Helper()
	root = t.TempDir()
	refs = filepath.Join(root, "references")
	require.NoError(t, os.MkdirAll(refs, 0o755))
	schema = filepath.Join(root, "prpt.schema.json")
	writeFile(t, schema, string(prptschema.Prpt))
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
	// role=composite without pl-* is rejected by the schema's if/then.
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: composite\nwhat:\n  is: x\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "workflow")
}

func TestCheckDirCompositeWithDispatchOK(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{
		"a.yml": "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\n        name: b\nwhat:\n  is: x\n",
		"b.yml": "frontmatter:\n  name: b\n  role: atom\nwhat:\n  is: x\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Empty(t, res.Issues)
}

func TestCheckDirCompositeDispatchDanglingDep(t *testing.T) {
	// dispatch step references a prompt that has no source file.
	_, refs, schema := setupLayout(t, map[string]string{
		"a.yml": "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\n        name: ghost\nwhat:\n  is: x\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `"ghost" has no prompt file`)
}

func TestCheckDirCompositeDispatchMissingKeys(t *testing.T) {
	// dispatch step without kind/name must fail schema (oneOf object requires both).
	_, refs, schema := setupLayout(t, map[string]string{
		"a.yml": "frontmatter:\n  name: a\n  role: composite\nworkflow:\n  - phase: x\n    steps:\n      - kind: prompt\nwhat:\n  is: x\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, "name")
}

func TestCheckDirStrictSectionKeys(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{
		// output.format is schema-valid; a bogus sub-key must be flagged.
		"ok.yml":  "frontmatter:\n  name: ok\n  role: atom\nwhat:\n  is: x\noutput:\n  format: md\n  template: x\n",
		"bad.yml": "frontmatter:\n  name: bad\n  role: atom\nwhat:\n  is: x\noutput:\n  format: md\n  bogus: 1\n",
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
	writeFile(t, filepath.Join(root, "prpt.schema.json"), string(prptschema.Prpt))
	writeFile(t, filepath.Join(refs, "a.yml"), samplePrompt)

	res, err := CheckDir(refs, filepath.Join(root, "prpt.schema.json"))
	require.NoError(t, err)
	assert.Empty(t, res.Issues)
}
