package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"

	"github.com/gookit/goutil/fsutil"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/pkg/utils"
	"github.com/xbpk3t/docs-alfred/service"
	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/task"
)

type FileType string

const (
	FileTypeJSON FileType = "json"
	FileTypeYAML FileType = "yml"
)

// DocProcessor 统一的处理器结构
type DocProcessor struct {
	Dst             string   `yaml:"dst"`             // 输出目录
	MergeOutputFile string   `yaml:"mergeOutputFile"` // 合并后的输出文件名
	currentFile     string   // 当前处理的文件名
	fileType        FileType // 内部字段，指定文件类型
}

// DocsConfig 定义配置结构
type DocsConfig struct {
	JSON  *DocProcessor `yaml:"json"`
	YAML  *DocProcessor `yaml:"yaml"`
	Src   string        `yaml:"src"` // 源路径
	Cmd   string        `yaml:"cmd"` // 命令类型
	IsDir bool          `yaml:"-"`   // 是否为文件夹，根据src自动判断
}

// getServiceParseModeMap returns the parse mode mapping for different service types
func getServiceParseModeMap() map[service.ServiceType]render.ParseMode {
	return map[service.ServiceType]render.ParseMode{
		service.ServiceGoods:  render.ParseFlatten,
		service.ServiceTask:   render.ParseMulti,
		service.ServiceGithub: render.ParseFlatten,
	}
}

// NewDocProcessor 创建新的处理器
func NewDocProcessor(fileType FileType) *DocProcessor {
	return &DocProcessor{
		fileType: fileType,
	}
}

// NewDocsConfig 创建新的配置实例
func NewDocsConfig(src, cmd string) *DocsConfig {
	return &DocsConfig{
		JSON: NewDocProcessor(FileTypeJSON),
		YAML: NewDocProcessor(FileTypeYAML),
		Src:  src,
		Cmd:  cmd,
	}
}

// DocProcessor 的方法实现
func (p *DocProcessor) SetCurrentFile(filename string) {
	p.currentFile = filename
}

func (p *DocProcessor) GetCurrentFile() string {
	return p.currentFile
}

func (p *DocProcessor) getOutputFilename(src string) string {
	if p.MergeOutputFile != "" {
		return p.MergeOutputFile
	}

	base := filepath.Base(src)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return name + "." + string(p.fileType)
}

func (p *DocProcessor) ProcessFile(src string, renderer render.Renderer) error {
	data, err := p.ReadInput(src, fsutil.IsDir(src))
	if err != nil {
		slog.Error("read file error",
			slog.String("file", src),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("read file error: %w", err)
	}

	content, err := renderer.Render(data)
	if err != nil {
		slog.Error("render error",
			slog.String("file", src),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("render error: %w", err)
	}

	// 如果需要 JSON 输出，将 YAML 转换为 JSON
	if p.fileType == FileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			slog.Error("convert to json error",
				slog.String("file", src),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("convert to json error: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := p.getOutputFilename(src)
	if err := p.WriteOutput(content, outputFilename); err != nil {
		slog.Error("write file error",
			slog.String("file", outputFilename),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

func (p *DocProcessor) ReadInput(src string, isDir bool) ([]byte, error) {
	if isDir {
		return p.readAndMergeFiles(src)
	}
	return p.readSingleFile(src)
}

func (p *DocProcessor) readSingleFile(src string) ([]byte, error) {
	if fsutil.IsDir(src) {
		return []byte(""), fmt.Errorf("stat path error")
	}
	return utils.ReadSingleFileWithExt(src, p.SetCurrentFile)
}

func (p *DocProcessor) readAndMergeFiles(src string) ([]byte, error) {
	if !fsutil.IsDir(src) {
		return []byte(""), fmt.Errorf("stat path error")
	}
	return utils.ReadAndMergeFilesRecursively(src, p.SetCurrentFile)
}

func (p *DocProcessor) WriteOutput(content string, filename string) error {
	if err := os.MkdirAll(p.Dst, 0o750); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(p.Dst, filename)
	if err := os.WriteFile(outputPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// Process 处理配置
func (dc *DocsConfig) Process() error {
	if err := dc.initializePath(); err != nil {
		return err
	}

	processors := dc.getProcessors()
	return dc.processAll(processors)
}

// initializePath 初始化路径相关的设置
func (dc *DocsConfig) initializePath() error {
	// 获取绝对路径
	absPath, err := filepath.Abs(dc.Src)
	if err != nil {
		return fmt.Errorf("get absolute path error: %w", err)
	}
	dc.Src = absPath

	// 检查路径是否存在并设置IsDir
	fileInfo, err := os.Stat(dc.Src)
	if err != nil {
		return fmt.Errorf("stat path error: %w", err)
	}
	dc.IsDir = fileInfo.IsDir()
	return nil
}

// getProcessors 获取所有处理器
func (dc *DocsConfig) getProcessors() map[FileType]*DocProcessor {
	return map[FileType]*DocProcessor{
		FileTypeJSON: dc.JSON,
		FileTypeYAML: dc.YAML,
	}
}

// processAll 处理所有文件
func (dc *DocsConfig) processAll(processors map[FileType]*DocProcessor) error {
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

// processSingle 处理单个文件
func (dc *DocsConfig) processSingle(fileType FileType, processor *DocProcessor) error {
	// 创建对应的渲染器
	renderer, err := dc.createRenderer()
	if err != nil {
		slog.Error("create renderer error",
			slog.String("type", string(fileType)),
			slog.String("file", dc.Src),
		)
		return fmt.Errorf("create renderer error for %s: %w", fileType, err)
	}

	// 处理文件
	if err := processor.ProcessFile(dc.Src, renderer); err != nil {
		slog.Error("process file error",
			slog.String("type", string(fileType)),
			slog.String("file", dc.Src),
		)
		return fmt.Errorf("process %s error: %w", fileType, err)
	}

	return nil
}

func (dc *DocsConfig) createRenderer() (render.Renderer, error) {
	// 根据命令类型选择渲染器
	var renderer render.Renderer
	switch dc.Cmd {
	case "task":
		renderer = task.NewTaskYAMLRender()
	case "gh":
		renderer = gh.NewGithubYAMLRender()
	case "goods":
		renderer = goods.NewGoodsYAMLRender()
	default:
		renderer = render.NewYAMLRenderer(dc.Cmd, true)
	}

	return dc.configureRenderer(renderer)
}

func (dc *DocsConfig) configureRenderer(renderer render.Renderer) (render.Renderer, error) {
	if err := dc.configureParseMode(renderer); err != nil {
		return nil, err
	}
	return renderer, nil
}

// configureParseMode 配置渲染器的解析模式
func (dc *DocsConfig) configureParseMode(renderer any) error {
	type parseModeRenderer interface {
		WithParseMode(mode render.ParseMode)
	}

	if r, ok := renderer.(parseModeRenderer); ok {
		serviceParseModeMap := getServiceParseModeMap()
		parseMode, exists := serviceParseModeMap[service.ServiceType(dc.Cmd)]
		if !exists {
			parseMode = render.ParseSingle
		}
		r.WithParseMode(parseMode)
		return nil
	}
	return fmt.Errorf("renderer does not support parse mode configuration")
}
