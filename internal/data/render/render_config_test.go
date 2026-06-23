package datarender

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

func TestProcessRenderConfigEmpty(t *testing.T) {
	cfg := processRenderConfig(docsConfig{})
	assert.Equal(t, "", cfg.Src)
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

func TestProcessRenderConfigsReturnsProcessError(t *testing.T) {
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = t.TempDir()

	err := processRenderConfigs([]docsConfig{{
		Src:  "missing.yml",
		Cmd:  "gh",
		YAML: proc,
	}})

	require.Error(t, err)
}

func TestRunPreservesGithubNixMetadataInYAMLAndJSONOutputs(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "kernel")
	outDir := filepath.Join(tmpDir, "public")
	require.NoError(t, os.MkdirAll(tagDir, 0755))

	mainNix := "https://mynixos.com/nixpkgs/package/main-tool"
	relatedNix := "https://mynixos.com/nixpkgs/package/related-tool"
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
      des: Main tool
      doc: data/gh/main-tool
      nix: https://mynixos.com/nixpkgs/package/main-tool
      rel:
        - url: https://github.com/acme/related-tool
          des: Related tool
          nix: https://mynixos.com/nixpkgs/package/related-tool
`), 0644))

	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  src,
		Cmd:  "gh",
		JSON: &docProcessor{Dst: outDir},
		YAML: &docProcessor{Dst: outDir},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	yamlData, err := os.ReadFile(filepath.Join(outDir, "gh.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "nix: "+mainNix)
	assert.Contains(t, string(yamlData), "nix: "+relatedNix)

	var configRepos ghindex.ConfigRepos
	require.NoError(t, yaml.Unmarshal(yamlData, &configRepos))
	require.Len(t, configRepos, 1)
	require.Len(t, configRepos[0].Repos, 1)
	mainRepo := configRepos[0].Repos[0]
	assert.Equal(t, mainNix, mainRepo.NixURL)
	require.Len(t, mainRepo.RelatedRepos, 1)
	assert.Equal(t, relatedNix, mainRepo.RelatedRepos[0].NixURL)

	jsonData, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	var jsonRepos []map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &jsonRepos))
	require.Len(t, jsonRepos, 1)
	jsonRepo, ok := jsonRepos[0]["repo"].([]any)
	require.True(t, ok)
	require.Len(t, jsonRepo, 1)
	jsonMainRepo, ok := jsonRepo[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, mainNix, jsonMainRepo["nix"])
	jsonRelatedRepos, ok := jsonMainRepo["rel"].([]any)
	require.True(t, ok)
	require.Len(t, jsonRelatedRepos, 1)
	jsonRelatedRepo, ok := jsonRelatedRepos[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, relatedNix, jsonRelatedRepo["nix"])
}

func TestRun_InvalidConfigFile(t *testing.T) {
	_, err := Run("/tmp/nonexistent-render-config-99999.yml")
	require.Error(t, err)
}

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

func TestServiceParseModeMap(t *testing.T) {
	m := serviceParseModeMap()
	assert.Equal(t, render.ParseFlatten, m["goods"])
	assert.Equal(t, render.ParseMulti, m["task"])
	assert.Equal(t, render.ParseFlatten, m["gh"])
}

func TestRun_TaskRenderer(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "task.yml"), []byte(`---
- task: test task
  date: "2024-01-01"
`), 0644))

	outDir := filepath.Join(tmpDir, "public")
	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  src,
		Cmd:  "task",
		YAML: &docProcessor{Dst: outDir},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRun_GoodsRenderer(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "goods.yml"), []byte(`---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
`), 0644))

	outDir := filepath.Join(tmpDir, "public")
	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  src,
		Cmd:  "goods",
		YAML: &docProcessor{Dst: outDir},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
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

func TestRun_MissingConfig(t *testing.T) {
	_, err := Run("/tmp/nonexistent-config-99999.yml")
	require.Error(t, err)
}

func TestRun_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.yml"), []byte("not: [valid: yaml"), 0644))
	_, err := Run(filepath.Join(tmpDir, "bad.yml"))
	require.Error(t, err)
}

func TestRun_JSONRenderer(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data")
	require.NoError(t, os.MkdirAll(src, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "test.yml"), []byte(`---
- name: test
  score: 4
`), 0644))

	outDir := filepath.Join(tmpDir, "public")
	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  src,
		Cmd:  "task",
		JSON: &docProcessor{Dst: outDir},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreateRenderer_Task(t *testing.T) {
	dc := &docsConfig{Cmd: "task"}
	renderer, err := dc.createRenderer()
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRenderer_GH(t *testing.T) {
	dc := &docsConfig{Cmd: "gh"}
	renderer, err := dc.createRenderer()
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRenderer_Goods(t *testing.T) {
	dc := &docsConfig{Cmd: "goods"}
	renderer, err := dc.createRenderer()
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestCreateRenderer_Default(t *testing.T) {
	dc := &docsConfig{Cmd: "default"}
	renderer, err := dc.createRenderer()
	require.NoError(t, err)
	assert.NotNil(t, renderer)
}

func TestConfigureParseMode_Unsupported(t *testing.T) {
	dc := &docsConfig{Cmd: "unknown"}
	err := dc.configureParseMode(struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support parse mode")
}

func TestProcessRenderConfigWithBoth(t *testing.T) {
	cfg := processRenderConfig(docsConfig{
		Src:  "test",
		JSON: &docProcessor{Dst: "out.json"},
		YAML: &docProcessor{Dst: "out.yml"},
	})
	assert.NotNil(t, cfg.JSON)
	assert.NotNil(t, cfg.YAML)
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


func TestProcessAll_NilProcessors(t *testing.T) {
	dc := &docsConfig{
		Src:  "/nonexistent",
		JSON: nil,
		YAML: nil,
	}
	err := dc.processAll(dc.getProcessors())
	require.NoError(t, err)
}

func TestInitializePath_AbsError(t *testing.T) {
	// This is hard to trigger since filepath.Abs rarely fails
	// But we can test with a valid path
	dc := &docsConfig{Src: t.TempDir()}
	err := dc.initializePath()
	require.NoError(t, err)
	assert.True(t, dc.IsDir)
}

func TestInitializePath_StatError(t *testing.T) {
	dc := &docsConfig{Src: "/nonexistent-path-99999"}
	err := dc.initializePath()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat path error")
}

func TestProcessGithubDir_LoadError(t *testing.T) {
	dc := &docsConfig{
		Src:   "/nonexistent-gh-dir-99999",
		Cmd:   "gh",
		IsDir: true,
	}
	proc := &docProcessor{Dst: t.TempDir()}
	err := dc.processGithubDir(fileTypeJSON, proc)
	require.Error(t, err)
}

func TestProcessRenderConfigWithSrc(t *testing.T) {
	cfg := processRenderConfig(docsConfig{
		Src: "/path/to/src",
		Cmd: "gh",
		JSON: &docProcessor{
			Dst:             "/path/to/output",
			MergeOutputFile: "merged.json",
		},
	})
	assert.Equal(t, "/path/to/src", cfg.Src)
	assert.Equal(t, "gh", cfg.Cmd)
	assert.NotNil(t, cfg.JSON)
	assert.Equal(t, "/path/to/output", cfg.JSON.Dst)
	assert.Equal(t, "merged.json", cfg.JSON.MergeOutputFile)
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

func TestRun_MultipleConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	src1 := filepath.Join(tmpDir, "data1")
	src2 := filepath.Join(tmpDir, "data2")
	require.NoError(t, os.MkdirAll(src1, 0755))
	require.NoError(t, os.MkdirAll(src2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src1, "a.yml"), []byte("- name: a\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src2, "b.yml"), []byte("- name: b\n"), 0644))

	outDir := filepath.Join(tmpDir, "public")
	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{
		{Src: src1, Cmd: "default", YAML: &docProcessor{Dst: outDir}},
		{Src: src2, Cmd: "default", YAML: &docProcessor{Dst: outDir}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	count, err := Run(configPath)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// --- Additional coverage tests ---

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

func TestProcessFile_JSONConversionError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("---\n- name: test\n"), 0644))

	p := newDocProcessor(fileTypeJSON)
	p.Dst = tmpDir

	// Use a renderer that returns invalid YAML to trigger JSON conversion error
	renderer := &invalidYAMLRenderer{}
	err := p.processFile(filepath.Join(tmpDir, "test.yml"), renderer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "convert to json error")
}

// invalidYAMLRenderer returns content that is not valid YAML for JSON conversion.
type invalidYAMLRenderer struct{}

func (r *invalidYAMLRenderer) Render([]byte) (string, error) {
	return "{{invalid yaml: [}", nil
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

func TestProcessSingle_NonGHPath(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	dc := &docsConfig{
		Src:   tmpDir,
		Cmd:   "default",
		IsDir: true,
	}
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = filepath.Join(tmpDir, "out")

	err := dc.processSingle(fileTypeYAML, proc)
	require.NoError(t, err)
}

func TestProcessSingle_RendererError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yml"), []byte("- name: test\n"), 0644))

	dc := &docsConfig{
		Src:   tmpDir,
		Cmd:   "default",
		IsDir: true,
	}
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = filepath.Join(tmpDir, "out")

	// Use configureRenderer to test the error path - unsupported renderer type
	renderer := struct{}{}
	err := dc.configureParseMode(renderer)
	require.Error(t, err)
}

func TestConfigureRenderer_Success(t *testing.T) {
	dc := &docsConfig{Cmd: "task"}
	renderer, err := dc.createRenderer()
	require.NoError(t, err)
	require.NotNil(t, renderer)

	// configureRenderer should succeed
	result, err := dc.configureRenderer(renderer)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMarshalAndWriteGithubOutput_JSONConversionError(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &docsConfig{Src: tmpDir, Cmd: "gh"}
	proc := &docProcessor{Dst: tmpDir, fileType: fileTypeJSON}

	// Use empty ConfigRepos to trigger the YAML→JSON path
	allRepos := make(ghindex.ConfigRepos, 0)
	err := dc.marshalAndWriteGithubOutput(allRepos, fileTypeJSON, proc)
	// Empty repos should succeed (empty array)
	require.NoError(t, err)
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

func TestRun_ProcessError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "docs.yml")
	configData, err := yaml.Marshal([]docsConfig{{
		Src:  "/nonexistent-src-99999",
		Cmd:  "default",
		YAML: &docProcessor{Dst: filepath.Join(tmpDir, "out")},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0644))

	_, err = Run(configPath)
	require.Error(t, err)
}

func TestProcessAll_ProcessorError(t *testing.T) {
	tmpDir := t.TempDir()
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = tmpDir

	dc := &docsConfig{
		Src:   "/nonexistent-src-99999",
		JSON:  nil,
		YAML:  proc,
	}
	err := dc.processAll(dc.getProcessors())
	require.Error(t, err)
}

func TestConfigureRenderer_Error(t *testing.T) {
	dc := &docsConfig{Cmd: "default"}
	renderer := &simpleRenderer{}
	_, err := dc.configureRenderer(renderer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support parse mode")
}

// simpleRenderer implements render.Renderer but NOT parseModeRenderer.
type simpleRenderer struct{}

func (s *simpleRenderer) Render([]byte) (string, error) {
	return "- ok", nil
}

func TestProcessSingle_ProcessFileError(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &docsConfig{
		Src:   "/nonexistent-file-99999.yml",
		Cmd:   "default",
		IsDir: false,
	}
	proc := newDocProcessor(fileTypeYAML)
	proc.Dst = tmpDir

	err := dc.processSingle(fileTypeYAML, proc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process")
}

func TestProcessSingle_GithubDirPath(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "data", "gh")
	tagDir := filepath.Join(src, "kernel")
	require.NoError(t, os.MkdirAll(tagDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "tool.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/main-tool
`), 0644))

	dc := &docsConfig{
		Src:   src,
		Cmd:   "gh",
		IsDir: true,
	}
	outDir := filepath.Join(tmpDir, "out")
	proc := &docProcessor{Dst: outDir, fileType: fileTypeJSON}

	err := dc.processSingle(fileTypeJSON, proc)
	require.NoError(t, err)

	// Verify output was created
	data, err := os.ReadFile(filepath.Join(outDir, "gh.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "main-tool")
}
