package skx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validSchema = `---
frontmatter:
  name: x
  role: atom
  desc: d
  pl-serial:
  pl-parallel:
  status: active
  is-save: true
what:
  is:
  not:
gate:
constraint:
  must:
  must-not:
input:
workflow:
output:
self-check:
hint:
`

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
	schema = filepath.Join(root, "prpt.yml")
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
	assert.Contains(t, res.Issues[0].Message, "frontmatter.name")
}

func TestCheckDirUnknownTopLevelKey(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: atom\nbogus:\n  x: 1\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `unknown top-level key "bogus"`)
}

func TestCheckDirUnknownFrontmatterKey(t *testing.T) {
	_, refs, schema := setupLayout(t, map[string]string{"a.yml": "frontmatter:\n  name: a\n  role: atom\n  nope: 1\n"})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	assert.Len(t, res.Issues, 1)
	assert.Contains(t, res.Issues[0].Message, `unknown frontmatter key "nope"`)
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
		// output.rules is schema-valid; a bogus sub-key must be flagged.
		"ok.yml":  "frontmatter:\n  name: ok\n  role: atom\noutput:\n  format: md\n  rules:\n    - a\n    - b\n",
		"bad.yml": "frontmatter:\n  name: bad\n  role: atom\noutput:\n  format: md\n  bogus: 1\n",
	})

	res, err := CheckDir(refs, schema)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1, "only the schema-invalid sub-key should be flagged")
	assert.Contains(t, res.Issues[0].Message, `unknown output key "bogus"`)

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
	assert.Contains(t, res.Issues[0].Message, `unknown workflow item key "bloop"`)
}

func TestCheckSchemaDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prpt.yml"), "brand-new:\n  x: 1\n")

	_, err := CheckDir(dir, filepath.Join(dir, "prpt.yml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown key "brand-new"`)
}

func TestCheckDirFlatSchemaOK(t *testing.T) {
	// prpt.yml declares frontmatter keys flat (top level) — must be accepted.
	root := t.TempDir()
	refs := filepath.Join(root, "references")
	require.NoError(t, os.MkdirAll(refs, 0o755))
	flatSchema := "name: x\nrole: atom\ndesc: d\nis-save: true\nwhat:\n  is:\n  not:\n"
	writeFile(t, filepath.Join(root, "prpt.yml"), flatSchema)
	writeFile(t, filepath.Join(refs, "a.yml"), samplePrompt)

	res, err := CheckDir(refs, filepath.Join(root, "prpt.yml"))
	require.NoError(t, err)
	assert.Empty(t, res.Issues)
}

func TestFindSchema(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "prpt.yml")
	writeFile(t, schema, validSchema)

	got, err := FindSchema(filepath.Join(dir, "references", "analysis"))
	require.NoError(t, err)
	assert.Equal(t, schema, got)
}
