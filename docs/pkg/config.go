package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/goutil/fsutil"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/pkg/utils"
	"github.com/xbpk3t/docs-alfred/service"
)

type FileType string

const (
	FileTypeMarkdown FileType = "md"
	FileTypeJSON     FileType = "json"
	FileTypeYAML     FileType = "yml"
)

// DocProcessor 统一的处理器结构
type DocProcessor struct {
	Dst             string   `yaml:"dst"`             // 输出目录
	MergeOutputFile string   `yaml:"mergeOutputFile"` // 合并后的输出文件名
	currentFile     string   // 当前处理的文件名
	fileType        FileType // 内部字段，指定文件类型
	Exclude         []string `yaml:"exclude"` // 排除的文件
}

// DocsConfig 定义配置结构
type DocsConfig struct {
	Markdown *DocProcessor `yaml:"md"`
	JSON     *DocProcessor `yaml:"json"`
	YAML     *DocProcessor `yaml:"yaml"`
	Src      string        `yaml:"src"` // 源路径
	Cmd      string        `yaml:"cmd"` // 命令类型
	IsDir    bool          `yaml:"-"`   // 是否为文件夹，根据src自动判断
}

var serviceParseModeMap = map[service.ServiceType]render.ParseMode{
	service.ServiceGoods:  render.ParseMulti,
	service.ServiceWiki:   render.ParseMulti,
	service.ServiceTask:   render.ParseMulti,
	service.ServiceGithub: render.ParseMulti,
	service.ServiceBooks:  render.ParseMulti,
	service.ServiceVideo:  render.ParseMulti,
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
		Markdown: NewDocProcessor(FileTypeMarkdown),
		JSON:     NewDocProcessor(FileTypeJSON),
		YAML:     NewDocProcessor(FileTypeYAML),
		Src:      src,
		Cmd:      cmd,
	}
}

// DocProcessor 的方法实现
func (p *DocProcessor) SetCurrentFile(filename string) {
	p.currentFile = filename
}

func (p *DocProcessor) GetCurrentFile() string {
	return p.currentFile
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
	return utils.ReadAndMergeFilesRecursively(src, p.Exclude, p.SetCurrentFile)
}

func (p *DocProcessor) WriteOutput(content string, filename string) error {
	if err := os.MkdirAll(p.Dst, os.ModePerm); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(p.Dst, filename)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
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

// DocsConfig 的方法实现
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
		FileTypeMarkdown: dc.Markdown,
		FileTypeJSON:     dc.JSON,
		FileTypeYAML:     dc.YAML,
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
	renderer, err := dc.createRenderer(fileType)
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

func (dc *DocsConfig) createRenderer(fileType FileType) (render.Renderer, error) {
	switch fileType {
	case FileTypeJSON:
		return dc.configureRenderer(render.NewJSONRenderer(dc.Cmd, true))
	case FileTypeYAML:
		return dc.configureRenderer(render.NewYAMLRenderer(dc.Cmd, true))
	}
	return nil, fmt.Errorf("unknown file type: %s", fileType)
}

func (dc *DocsConfig) configureRenderer(renderer render.Renderer) (render.Renderer, error) {
	if err := dc.configureParseMode(renderer); err != nil {
		return nil, err
	}
	return renderer, nil
}

// configureParseMode 配置渲染器的解析模式
func (dc *DocsConfig) configureParseMode(renderer interface{}) error {
	type parseModeRenderer interface {
		WithParseMode(mode render.ParseMode)
	}

	if r, ok := renderer.(parseModeRenderer); ok {
		parseMode, exists := serviceParseModeMap[service.ServiceType(dc.Cmd)]
		if !exists {
			parseMode = render.ParseSingle
		}
		r.WithParseMode(parseMode)
		return nil
	}
	return fmt.Errorf("renderer does not support parse mode configuration")
}
