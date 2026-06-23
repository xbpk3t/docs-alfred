package datarender

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/internal/gh/task"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

type fileType string

const (
	fileTypeJSON fileType = "json"
	fileTypeYAML fileType = "yml"
)

// DomainRenderConfig holds configuration for rendering a single domain.
type DomainRenderConfig struct {
	Domain string
	Src    string
	OutDir string
	Format string // "json", "yaml", "json,yaml"
}

// DomainRenderResult holds the result of a domain render.
type DomainRenderResult struct {
	OutputFiles []string
}

// RunDomainRender renders a single domain's data into the specified output formats.
func RunDomainRender(cfg DomainRenderConfig) (*DomainRenderResult, error) {
	src, err := filepath.Abs(cfg.Src)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	fi, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("stat source path %s: %w", src, err)
	}

	isSourceDir := fi.IsDir()
	formats := strings.Split(cfg.Format, ",")

	renderer, err := createRendererForDomain(cfg.Domain)
	if err != nil {
		return nil, err
	}

	var outputFiles []string

	for _, f := range formats {
		f = strings.TrimSpace(f)
		ft := normalizeFormat(f)
		if ft == "" {
			return nil, fmt.Errorf("unsupported format %q", f)
		}

		proc := newDocProcessor(ft)
		proc.Dst = cfg.OutDir

		if cfg.Domain == "gh" && isSourceDir {
			if err := processGithubDirDomain(src, ft, proc); err != nil {
				return nil, fmt.Errorf("process gh dir: %w", err)
			}
		} else {
			if err := proc.processFile(src, renderer); err != nil {
				return nil, fmt.Errorf("process %s: %w", ft, err)
			}
		}

		outputFiles = append(outputFiles, filepath.Join(cfg.OutDir, proc.getOutputFilename(src)))
	}

	return &DomainRenderResult{OutputFiles: outputFiles}, nil
}

// createRendererForDomain returns the appropriate renderer for a domain.
func createRendererForDomain(domain string) (render.Renderer, error) {
	var renderer render.Renderer
	switch domain {
	case "task":
		renderer = task.NewTaskYAMLRender()
	case "gh":
		renderer = ghindex.NewGithubYAMLRender("")
	case "goods":
		renderer = goods.NewGoodsYAMLRender()
	default:
		renderer = render.NewYAMLRenderer(domain, true)
	}

	parseMode, exists := serviceParseModeMap()[domain]
	if !exists {
		parseMode = render.ParseSingle
	}

	type parseModeRenderer interface {
		WithParseMode(mode render.ParseMode)
	}

	r, ok := renderer.(parseModeRenderer)
	if !ok {
		return nil, errors.New("renderer does not support parse mode configuration")
	}
	r.WithParseMode(parseMode)

	return renderer, nil
}

// processGithubDirDomain handles the gh domain's special directory-based rendering.
func processGithubDirDomain(src string, ft fileType, proc *docProcessor) error {
	allRepos, err := ghindex.LoadConfigReposFromDir(src)
	if err != nil {
		return err
	}

	result, err := yaml.Marshal(allRepos)
	if err != nil {
		return fmt.Errorf("marshal gh repos: %w", err)
	}

	content := string(result)
	if ft == fileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			return fmt.Errorf("convert gh to json: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := proc.getOutputFilename(src)
	if err := proc.writeOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write gh output: %w", err)
	}

	return nil
}

// normalizeFormat converts user-facing format names to internal fileType.
func normalizeFormat(f string) fileType {
	switch f {
	case "json":
		return fileTypeJSON
	case "yaml", "yml":
		return fileTypeYAML
	default:
		return ""
	}
}

// serviceParseModeMap returns the parse mode for each domain.
func serviceParseModeMap() map[string]render.ParseMode {
	return map[string]render.ParseMode{
		"goods": render.ParseFlatten,
		"task":  render.ParseMulti,
		"gh":    render.ParseFlatten,
	}
}

// ---------------------------------------------------------------------------
// docProcessor — file I/O and output helpers
// ---------------------------------------------------------------------------

type docProcessor struct {
	Dst             string `yaml:"dst"`
	MergeOutputFile string `yaml:"mergeOutputFile"`
	currentFile     string
	fileType        fileType
}

func newDocProcessor(fileType fileType) *docProcessor {
	return &docProcessor{fileType: fileType}
}

func (p *docProcessor) setCurrentFile(filename string) {
	p.currentFile = filename
}

func (p *docProcessor) getOutputFilename(src string) string {
	if p.MergeOutputFile != "" {
		return p.MergeOutputFile
	}

	base := filepath.Base(src)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	return name + "." + string(p.fileType)
}

func (p *docProcessor) processFile(src string, renderer render.Renderer) error {
	data, err := p.readInput(src, isDir(src))
	if err != nil {
		slog.Error("read file error", "file", src, "error", err.Error())

		return fmt.Errorf("read file error: %w", err)
	}

	content, err := renderer.Render(data)
	if err != nil {
		slog.Error("render error", "file", src, "error", err.Error())

		return fmt.Errorf("render error: %w", err)
	}

	if p.fileType == fileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			slog.Error("convert to json error", "file", src, "error", err.Error())

			return fmt.Errorf("convert to json error: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := p.getOutputFilename(src)
	if err := p.writeOutput(content, outputFilename); err != nil {
		slog.Error("write file error", "file", outputFilename, "error", err.Error())

		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

func (p *docProcessor) readInput(src string, isDir bool) ([]byte, error) {
	if isDir {
		return p.readAndMergeFiles(src)
	}

	return p.readSingleFile(src)
}

func (p *docProcessor) readSingleFile(src string) ([]byte, error) {
	if isDir(src) {
		return []byte(""), errors.New("stat path error")
	}

	return fileutil.ReadSingleFile(src, p.setCurrentFile)
}

func (p *docProcessor) readAndMergeFiles(src string) ([]byte, error) {
	if !isDir(src) {
		return []byte(""), errors.New("stat path error")
	}

	return fileutil.ReadAndMergeYAMLFilesRecursive(src, p.setCurrentFile)
}

func isDir(path string) bool {
	fi, err := os.Stat(path)

	return err == nil && fi.IsDir()
}

func (p *docProcessor) writeOutput(content, filename string) error {
	if err := fileutil.EnsureDir(p.Dst); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(p.Dst, filename)
	if err := fileutil.AtomicWriteFile(outputPath, []byte(content), fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Legacy support — docsConfig is kept for marshalAndWriteGithubOutput helper
// ---------------------------------------------------------------------------

type docsConfig struct {
	JSON  *docProcessor `yaml:"json"`
	YAML  *docProcessor `yaml:"yaml"`
	Src   string        `yaml:"src"`
	Cmd   string        `yaml:"cmd"`
	IsDir bool          `yaml:"-"`
}

func processRenderConfig(raw docsConfig) docsConfig {
	config := docsConfig{
		Src: raw.Src,
		Cmd: raw.Cmd,
	}
	if raw.JSON != nil {
		config.JSON = newDocProcessor(fileTypeJSON)
		config.JSON.Dst = raw.JSON.Dst
		config.JSON.MergeOutputFile = raw.JSON.MergeOutputFile
	}
	if raw.YAML != nil {
		config.YAML = newDocProcessor(fileTypeYAML)
		config.YAML.Dst = raw.YAML.Dst
		config.YAML.MergeOutputFile = raw.YAML.MergeOutputFile
	}

	return config
}

func (dc *docsConfig) marshalAndWriteGithubOutput(
	allRepos ghindex.ConfigRepos,
	fileType fileType,
	processor *docProcessor,
) error {
	result, err := yaml.Marshal(allRepos)
	if err != nil {
		return fmt.Errorf("marshal gh repos error: %w", err)
	}

	content := string(result)
	if fileType == fileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			return fmt.Errorf("convert gh to json error: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := processor.getOutputFilename(dc.Src)
	if err := processor.writeOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write gh output error: %w", err)
	}

	return nil
}
