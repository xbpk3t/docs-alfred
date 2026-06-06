package datarender

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	rootpkg "github.com/xbpk3t/docs-alfred/pkg"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
	servicegh "github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/task"
)

type fileType string

const (
	fileTypeJSON fileType = "json"
	fileTypeYAML fileType = "yml"
)

type docProcessor struct {
	Dst             string `yaml:"dst"`
	MergeOutputFile string `yaml:"mergeOutputFile"`
	currentFile     string
	fileType        fileType
}

type docsConfig struct {
	JSON  *docProcessor `yaml:"json"`
	YAML  *docProcessor `yaml:"yaml"`
	Src   string        `yaml:"src"`
	Cmd   string        `yaml:"cmd"`
	IsDir bool          `yaml:"-"`
}

// Run renders all configured data sources from cfgFile and returns the config count.
func Run(cfgFile string) (int, error) {
	configs, err := loadRenderConfigs(cfgFile)
	if err != nil {
		return 0, err
	}
	if err := processRenderConfigs(configs); err != nil {
		return 0, err
	}

	return len(configs), nil
}

func newDocProcessor(fileType fileType) *docProcessor {
	return &docProcessor{fileType: fileType}
}

func loadRenderConfigs(cfgFile string) ([]docsConfig, error) {
	configData, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var rawConfigs []docsConfig
	if err := yaml.NewDecoder(bytes.NewReader(configData)).Decode(&rawConfigs); err != nil {
		return nil, err
	}

	configs := make([]docsConfig, 0, len(rawConfigs))
	for i := range rawConfigs {
		configs = append(configs, processRenderConfig(rawConfigs[i]))
	}

	return configs, nil
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

func processRenderConfigs(configs []docsConfig) error {
	for i := range configs {
		if err := configs[i].process(); err != nil {
			return err
		}
	}

	return nil
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

	return rootpkg.ReadSingleFileWithExt(src, p.setCurrentFile)
}

func (p *docProcessor) readAndMergeFiles(src string) ([]byte, error) {
	if !isDir(src) {
		return []byte(""), errors.New("stat path error")
	}

	return rootpkg.ReadAndMergeFilesRecursively(src, p.setCurrentFile)
}

func isDir(path string) bool {
	fi, err := os.Stat(path)

	return err == nil && fi.IsDir()
}

func (p *docProcessor) writeOutput(content, filename string) error {
	if err := os.MkdirAll(p.Dst, fileutil.DirPerm); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(p.Dst, filename)
	if err := fileutil.AtomicWriteFile(outputPath, []byte(content), fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

func (dc *docsConfig) process() error {
	if err := dc.initializePath(); err != nil {
		return err
	}

	processors := dc.getProcessors()

	return dc.processAll(processors)
}

func (dc *docsConfig) initializePath() error {
	absPath, err := filepath.Abs(dc.Src)
	if err != nil {
		return fmt.Errorf("get absolute path error: %w", err)
	}
	dc.Src = absPath

	fileInfo, err := os.Stat(dc.Src)
	if err != nil {
		return fmt.Errorf("stat path error: %w", err)
	}
	dc.IsDir = fileInfo.IsDir()

	return nil
}

func (dc *docsConfig) getProcessors() map[fileType]*docProcessor {
	return map[fileType]*docProcessor{
		fileTypeJSON: dc.JSON,
		fileTypeYAML: dc.YAML,
	}
}

func (dc *docsConfig) processAll(processors map[fileType]*docProcessor) error {
	for fileType, processor := range processors {
		if processor == nil {
			continue
		}

		if err := dc.processSingle(fileType, processor); err != nil {
			return err
		}
	}

	return nil
}

func (dc *docsConfig) processSingle(fileType fileType, processor *docProcessor) error {
	if dc.Cmd == "gh" && dc.IsDir {
		return dc.processGithubDir(fileType, processor)
	}

	renderer, err := dc.createRenderer()
	if err != nil {
		slog.Error("create renderer error", "type", string(fileType), "file", dc.Src)

		return fmt.Errorf("create renderer error for %s: %w", fileType, err)
	}

	if err := processor.processFile(dc.Src, renderer); err != nil {
		slog.Error("process file error", "type", string(fileType), "file", dc.Src)

		return fmt.Errorf("process %s error: %w", fileType, err)
	}

	return nil
}

func (dc *docsConfig) processGithubDir(fileType fileType, processor *docProcessor) error {
	allRepos, err := servicegh.LoadConfigReposFromDir(dc.Src)
	if err != nil {
		return err
	}

	return dc.marshalAndWriteGithubOutput(allRepos, fileType, processor)
}

func (dc *docsConfig) marshalAndWriteGithubOutput(
	allRepos servicegh.ConfigRepos,
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

func (dc *docsConfig) createRenderer() (render.Renderer, error) {
	var renderer render.Renderer
	switch dc.Cmd {
	case "task":
		renderer = task.NewTaskYAMLRender()
	case "gh":
		renderer = servicegh.NewGithubYAMLRender("")
	case "goods":
		renderer = goods.NewGoodsYAMLRender()
	default:
		renderer = render.NewYAMLRenderer(dc.Cmd, true)
	}

	return dc.configureRenderer(renderer)
}

func (dc *docsConfig) configureRenderer(renderer render.Renderer) (render.Renderer, error) {
	if err := dc.configureParseMode(renderer); err != nil {
		return nil, err
	}

	return renderer, nil
}

func (dc *docsConfig) configureParseMode(renderer any) error {
	type parseModeRenderer interface {
		WithParseMode(mode render.ParseMode)
	}

	r, ok := renderer.(parseModeRenderer)
	if !ok {
		return errors.New("renderer does not support parse mode configuration")
	}

	parseMode, exists := serviceParseModeMap()[service.ServiceType(dc.Cmd)]
	if !exists {
		parseMode = render.ParseSingle
	}
	r.WithParseMode(parseMode)

	return nil
}

func serviceParseModeMap() map[service.ServiceType]render.ParseMode {
	return map[service.ServiceType]render.ParseMode{
		service.ServiceGoods:  render.ParseFlatten,
		service.ServiceTask:   render.ParseMulti,
		service.ServiceGithub: render.ParseFlatten,
	}
}
