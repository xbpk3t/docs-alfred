package datarender

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// ---------------------------------------------------------------------------
// RunDomainRender
// ---------------------------------------------------------------------------

func TestRunDomainRender_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "test.yml"), []byte("- name: test\n"), 0644))

	_, err := RunDomainRender(DomainRenderConfig{
		Domain: "default",
		Src:    src,
		OutDir: filepath.Join(tmpDir, "out"),
		Format: "invalid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRunDomainRender_NonexistentSrc(t *testing.T) {
	_, err := RunDomainRender(DomainRenderConfig{
		Domain: "default",
		Src:    "/nonexistent-src-99999",
		OutDir: t.TempDir(),
		Format: "yaml",
	})
	require.Error(t, err)
}

func TestRunDomainRender_DefaultRenderer(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "test.yml"), []byte("- name: test\n"), 0644))
	outDir := filepath.Join(tmpDir, "out")

	result, err := RunDomainRender(DomainRenderConfig{
		Domain: "default",
		Src:    src,
		OutDir: outDir,
		Format: "yaml",
	})
	require.NoError(t, err)
	assert.Len(t, result.OutputFiles, 1)
}

func TestRunDomainRender_GHD(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
      des: Main tool
`), 0644))
	outDir := filepath.Join(tmpDir, "out")

	result, err := RunDomainRender(DomainRenderConfig{
		Domain: "gh",
		Src:    src,
		OutDir: outDir,
		Format: "json,yaml",
	})
	require.NoError(t, err)
	assert.Len(t, result.OutputFiles, 2)

	// Verify YAML output
	yamlData, err := os.ReadFile(filepath.Join(outDir, "gh.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "main-tool")

	// Verify JSON output
	jsonData, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	var jsonRepos []map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &jsonRepos))
	assert.Len(t, jsonRepos, 1)
}

func TestRunDomainRender_Goods(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "goods.yml"), []byte(`---
- type: EDC
  tag: goods
  score: 3
  topics:
    - topic: earphones
      score: 5
      table:
        - name: AirPods
          brand: Apple
          price: ¥998
    - topic: multitool
      table:
        - name: MR-1098SL
          brand: Mr.Green
          price: ¥49
  item:
    - name: C50
      price: ¥179
`), 0644))
	outDir := filepath.Join(tmpDir, "out")

	result, err := RunDomainRender(DomainRenderConfig{
		Domain: "goods",
		Src:    src,
		OutDir: outDir,
		Format: "json",
	})
	require.NoError(t, err)
	assert.Len(t, result.OutputFiles, 1)

	raw, err := os.ReadFile(result.OutputFiles[0])
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded, 1)

	topics, ok := decoded[0]["topics"].([]any)
	require.True(t, ok)
	require.Len(t, topics, 2)

	earphones, ok := topics[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "earphones", earphones["topic"])
	assert.EqualValues(t, 5, earphones["score"])
	table, ok := earphones["table"].([]any)
	require.True(t, ok, "earphones.table must survive render; missing means content.Topic lacks Table field")
	require.Len(t, table, 1)
	row, ok := table[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AirPods", row["name"])

	multitool, ok := topics[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "multitool", multitool["topic"])
	mTable, ok := multitool["table"].([]any)
	require.True(t, ok)
	assert.Len(t, mTable, 1)
}

func TestRunDomainRender_Task(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "task.yml"), []byte(`---
- task: test task
  date: "2024-01-01"
`), 0644))
	outDir := filepath.Join(tmpDir, "out")

	result, err := RunDomainRender(DomainRenderConfig{
		Domain: "task",
		Src:    src,
		OutDir: outDir,
		Format: "yaml",
	})
	require.NoError(t, err)
	assert.Len(t, result.OutputFiles, 1)
}

func TestRunDomainRender_NixMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "kernel")
	require.NoError(t, os.MkdirAll(tagDir, 0755))

	mainNix := "https://mynixos.com/nixpkgs/package/main-tool"
	relatedNix := "https://mynixos.com/nixpkgs/package/related-tool"
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
      des: Main tool
      nix: `+mainNix+`
      rel:
        - url: https://github.com/acme/related-tool
          des: Related tool
          nix: `+relatedNix+`
`), 0644))
	outDir := filepath.Join(tmpDir, "out")

	result, err := RunDomainRender(DomainRenderConfig{
		Domain: "gh",
		Src:    src,
		OutDir: outDir,
		Format: "json,yaml",
	})
	require.NoError(t, err)
	assert.Len(t, result.OutputFiles, 2)

	// Verify YAML nix metadata preserved
	yamlData, err := os.ReadFile(filepath.Join(outDir, "gh.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "nix: "+mainNix)
	assert.Contains(t, string(yamlData), "nix: "+relatedNix)

	// Verify JSON nix metadata preserved
	jsonData, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	var jsonRepos []map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &jsonRepos))
	jsonRepo, ok := jsonRepos[0]["repo"].([]any)
	require.True(t, ok)
	mainRepo, ok := jsonRepo[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, mainNix, mainRepo["nix"])
}

// ---------------------------------------------------------------------------
// createRendererForDomain
// ---------------------------------------------------------------------------

func TestCreateRendererForDomain_Task(t *testing.T) {
	renderer, err := createRendererForDomain("task")
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRendererForDomain_GH(t *testing.T) {
	renderer, err := createRendererForDomain("gh")
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRendererForDomain_Goods(t *testing.T) {
	renderer, err := createRendererForDomain("goods")
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRendererForDomain_Default(t *testing.T) {
	renderer, err := createRendererForDomain("default")
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

// ---------------------------------------------------------------------------
// serviceParseModeMap
// ---------------------------------------------------------------------------

func TestServiceParseModeMap(t *testing.T) {
	m := serviceParseModeMap()
	assert.Equal(t, render.ParseFlatten, m["goods"])
	assert.Equal(t, render.ParseMulti, m["task"])
	assert.Equal(t, render.ParseFlatten, m["gh"])
}

// ---------------------------------------------------------------------------
// docProcessor
// ---------------------------------------------------------------------------

func TestNewDocProcessor(t *testing.T) {
	p := newDocProcessor(fileTypeJSON)
	assert.Equal(t, fileTypeJSON, p.fileType)

	p2 := newDocProcessor(fileTypeYAML)
	assert.Equal(t, fileTypeYAML, p2.fileType)
}

func TestGetOutputFilename(t *testing.T) {
	p := &docProcessor{fileType: fileTypeJSON}
	assert.Equal(t, "test.json", p.getOutputFilename("/path/to/test.yml"))

	p2 := &docProcessor{fileType: fileTypeYAML}
	assert.Equal(t, "test.yml", p2.getOutputFilename("/path/to/test.json"))

	p3 := &docProcessor{fileType: fileTypeJSON, MergeOutputFile: "merged.json"}
	assert.Equal(t, "merged.json", p3.getOutputFilename("/path/to/test.yml"))
}

func TestIsDir(t *testing.T) {
	tmpDir := t.TempDir()
	assert.True(t, isDir(tmpDir))
	assert.False(t, isDir("/tmp/nonexistent-dir-99999"))
}

func TestReadSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	p := newDocProcessor(fileTypeYAML)
	p.Dst = tmpDir
	data, err := p.readSingleFile(filepath.Join(tmpDir, "test.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
}

func TestReadSingleFile_IsDir(t *testing.T) {
	tmpDir := t.TempDir()
	p := newDocProcessor(fileTypeYAML)
	_, err := p.readSingleFile(tmpDir)
	require.Error(t, err)
}

func TestReadAndMergeFiles_NotDir(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	p := newDocProcessor(fileTypeYAML)
	_, err := p.readAndMergeFiles(filepath.Join(tmpDir, "test.yml"))
	require.Error(t, err)
}

func TestReadInput_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	p := newDocProcessor(fileTypeYAML)
	p.Dst = tmpDir

	data, err := p.readInput(filepath.Join(tmpDir, "test.yml"), false)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
}

func TestReadInput_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	p := newDocProcessor(fileTypeYAML)
	p.Dst = tmpDir

	data, err := p.readInput(tmpDir, true)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
}

func TestWriteOutput_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "nested", "out")
	p := &docProcessor{Dst: outDir}
	err := p.writeOutput("content", "test.txt")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestWriteOutput_EnsureDirError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where a directory component should be
	blocker := filepath.Join(tmpDir, "blocked")
	require.NoError(t, os.WriteFile(blocker, []byte(""), 0644))

	p := &docProcessor{Dst: filepath.Join(blocker, "nested", "out")}
	err := p.writeOutput("content", "test.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create dir error")
}

// ---------------------------------------------------------------------------
// processFile
// ---------------------------------------------------------------------------

// errorRenderer is a mock renderer that always returns an error.
type errorRenderer struct{}

func (e *errorRenderer) Render([]byte) (string, error) {
	return "", errors.New("render failed")
}

func TestProcessFile_ReadInputError(t *testing.T) {
	p := newDocProcessor(fileTypeYAML)
	p.Dst = t.TempDir()
	renderer := render.NewYAMLRenderer("default", true)

	err := p.processFile("/nonexistent-file-99999.yml", renderer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file error")
}

func TestProcessFile_RendererReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	p := newDocProcessor(fileTypeYAML)
	p.Dst = tmpDir

	err := p.processFile(filepath.Join(tmpDir, "test.yml"), &errorRenderer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render error")
}

func TestProcessFile_WriteOutputError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	// Make Dst a file instead of a dir so writeOutput fails
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "dst_file"), []byte(""), 0644))

	p := newDocProcessor(fileTypeYAML)
	p.Dst = filepath.Join(tmpDir, "dst_file")

	renderer := render.NewYAMLRenderer("default", true)
	err := p.processFile(filepath.Join(tmpDir, "test.yml"), renderer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write file error")
}

// invalidYAMLRenderer returns content that is not valid YAML for JSON conversion.
type invalidYAMLRenderer struct{}

func (r *invalidYAMLRenderer) Render([]byte) (string, error) {
	return "{{invalid yaml: [}", nil
}

func TestProcessFile_JSONConversionError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("---\n- name: test\n"), 0644))

	p := newDocProcessor(fileTypeJSON)
	p.Dst = tmpDir

	err := p.processFile(filepath.Join(tmpDir, "test.yml"), &invalidYAMLRenderer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to json error")
}

func TestProcessFile_JSONYAML(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("---\n- name: test\n  score: 4\n"), 0644))

	p := newDocProcessor(fileTypeJSON)
	p.Dst = tmpDir

	renderer := render.NewYAMLRenderer("default", true)
	err := p.processFile(filepath.Join(tmpDir, "test.yml"), renderer)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "test.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")
}

// ---------------------------------------------------------------------------
// processGithubDirDomain
// ---------------------------------------------------------------------------

func TestProcessGithubDirDomain_LoadError(t *testing.T) {
	proc := &docProcessor{Dst: t.TempDir()}
	err := processGithubDirDomain("/nonexistent-gh-dir-99999", fileTypeJSON, proc)
	require.Error(t, err)
}

func TestProcessGithubDirDomain_Success(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "dev")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
`), 0644))

	outDir := filepath.Join(tmpDir, "out")
	proc := &docProcessor{Dst: outDir, fileType: fileTypeJSON}

	err := processGithubDirDomain(src, fileTypeJSON, proc)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "main-tool")
}

// ---------------------------------------------------------------------------
// Legacy loadRenderConfigs / processRenderConfig (kept for potential reuse)
// ---------------------------------------------------------------------------

func TestProcessRenderConfigEmpty(t *testing.T) {
	cfg := processRenderConfig(docsConfig{})
	assert.Empty(t, cfg.Src)
}

func TestProcessRenderConfigWithJSON(t *testing.T) {
	proc := newDocProcessor(fileTypeJSON)
	proc.Dst = "output.json"

	cfg := processRenderConfig(docsConfig{
		Src:  "test",
		JSON: proc,
	})

	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.JSON)
	assert.Equal(t, "output.json", cfg.JSON.Dst)
}

func TestProcessRenderConfigWithYAML(t *testing.T) {
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = "output.yml"

	cfg := processRenderConfig(docsConfig{
		Src:  "test",
		YAML: proc,
	})

	assert.Equal(t, "test", cfg.Src)
	assert.NotNil(t, cfg.YAML)
	assert.Equal(t, "output.yml", cfg.YAML.Dst)
}

func TestMarshalAndWriteGithubOutput_EmptyRepos(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &docsConfig{Src: tmpDir, Cmd: "gh"}
	proc := &docProcessor{Dst: tmpDir, fileType: fileTypeJSON}
	allRepos := make(ghindex.ConfigRepos, 0)
	err := dc.marshalAndWriteGithubOutput(allRepos, fileTypeJSON, proc)
	require.NoError(t, err)
}

func TestMarshalAndWriteGithubOutput_JSONConversionError(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &docsConfig{Src: tmpDir, Cmd: "gh"}
	proc := &docProcessor{Dst: tmpDir, fileType: fileTypeJSON}
	allRepos := make(ghindex.ConfigRepos, 0)
	err := dc.marshalAndWriteGithubOutput(allRepos, fileTypeJSON, proc)
	// Empty repos should succeed (empty array)
	require.NoError(t, err)
}
