package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/diary"
	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	taskService "github.com/xbpk3t/docs-alfred/service/task"
	"github.com/xbpk3t/docs-alfred/service/works"
	"github.com/xbpk3t/docs-alfred/service/ws"
)

// DocsConfig 定义配置结构
type DocsConfig struct {
	Markdown *Markdown `yaml:"markdown"` // Using pointer to allow nil checks
	JSON     *JSON     `yaml:"json"`     // Using pointer to allow nil checks
	Src      string    `yaml:"src"`      // 源路径
	Cmd      string    `yaml:"cmd"`      // 命令类型
	IsDir    bool      `yaml:"-"`        // 是否为文件夹，根据src自动判断
}

type Markdown struct {
	Dst             string   `yaml:"dst"`
	MergeOutputFile string   `yaml:"mergeOutputFile"` // 合并后的输出文件名
	Exclude         []string `yaml:"exclude"`
	IsMerge         bool     `yaml:"isMerge"`
	IsRawLoad       bool     `yaml:"isRawLoad"` // 是否直接加载
	IsExpand        bool     `yaml:"isExpand"`  // 在docusaurus中是否展开
}

type JSON struct {
	Dst string `yaml:"dst"`
}

// NewDocsConfig 创建新的配置实例
func NewDocsConfig() *DocsConfig {
	return &DocsConfig{}
}

// Init 初始化配置
func (dc *DocsConfig) Init() error {
	// 获取绝对路径
	src, err := dc.getAbsPath()
	if err != nil {
		return fmt.Errorf("get absolute path error: %w", err)
	}
	dc.Src = src

	// 检查路径是否存在并设置IsDir
	fileInfo, err := os.Stat(dc.Src)
	if err != nil {
		return fmt.Errorf("stat path error: %w", err)
	}
	dc.IsDir = fileInfo.IsDir()

	return nil
}

// Process 处理配置
func (dc *DocsConfig) Process() error {
	if err := dc.Init(); err != nil {
		return err
	}

	// 处理 Markdown 输出
	if dc.Markdown != nil {
		if err := dc.parseMarkdown(); err != nil {
			return fmt.Errorf("parse Markdown error: %w", err)
		}
	}

	// 处理 JSON 输出
	if dc.JSON != nil {
		if dc.IsDir {
			if err := dc.parseJSON(); err != nil {
				return fmt.Errorf("parse JSON error: %w", err)
			}
		} else {
			if err := dc.parseJSONFile(); err != nil {
				return fmt.Errorf("parse JSON file error: %w", err)
			}
		}
	}

	return nil
}

// getAbsPath 获取绝对路径
func (dc *DocsConfig) getAbsPath() (string, error) {
	if filepath.IsAbs(dc.Src) {
		return dc.Src, nil
	}
	return filepath.Abs(dc.Src)
}

// createRenderer 创建渲染器
func (dc *DocsConfig) createRenderer() (render.Renderer, error) {
	// 如果配置了JSON输出，使用JSON渲染器
	if dc.JSON != nil {
		return render.NewJSONRenderer(dc.Cmd, true), nil
	}

	// 否则使用对应的Markdown渲染器
	switch dc.Cmd {
	case "works":
		return works.NewWorkRenderer(), nil
	case "gh":
		return gh.NewGithubMarkdownRender(), nil
	case "ws":
		return ws.NewWebStackRenderer(), nil
	case "diary":
		return diary.NewDiaryMarkdownRender(), nil
	case "goods":
		return goods.NewGoodsMarkdownRenderer(), nil
	case "task":
		return taskService.NewTaskRenderer(), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", dc.Cmd)
	}
}

// parseJSONFile 处理单个JSON文件
func (dc *DocsConfig) parseJSONFile() error {
	if dc.JSON == nil {
		return nil
	}

	// 读取单个文件
	data, err := os.ReadFile(dc.Src)
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		Src:        filepath.Dir(dc.Src),
		InputFile:  filepath.Base(dc.Src),
		TargetDir:  filepath.Dir(dc.JSON.Dst),
		OutputFile: filepath.Base(dc.JSON.Dst),
		IsMerge:    false, // 单文件模式不需要合并
	}

	// 创建渲染器并渲染
	renderer := render.NewJSONRenderer(dc.Cmd, true)
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// 写入文件
	if err := processor.WriteOutput(content); err != nil {
		return fmt.Errorf("write output error: %w", err)
	}

	return nil
}

// parseJSON 处理JSON配置
func (dc *DocsConfig) parseJSON() error {
	if dc.JSON == nil {
		return nil
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		Src:        dc.Src,
		TargetDir:  filepath.Dir(dc.JSON.Dst),
		OutputFile: filepath.Base(dc.JSON.Dst),
		IsMerge:    true, // JSON 输出总是合并模式
	}

	// 读取所有文件
	data, err := processor.ReadInput()
	if err != nil {
		return fmt.Errorf("read input error: %w", err)
	}

	// 创建渲染器并渲染
	renderer := render.NewJSONRenderer(dc.Cmd, true)
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// 写入文件
	if err := processor.WriteOutput(content); err != nil {
		return fmt.Errorf("write output error: %w", err)
	}

	return nil
}

// parseMarkdown 处理Markdown配置
func (dc *DocsConfig) parseMarkdown() error {
	if dc.Markdown == nil {
		return nil
	}

	// 获取目标路径
	targetDir := dc.Markdown.Dst
	if targetDir == "" {
		targetDir = "docs" // 默认输出到docs目录
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		Src:       dc.Src,
		TargetDir: targetDir,
		IsMerge:   dc.Markdown.IsMerge,
		Exclude:   dc.Markdown.Exclude,
	}

	// 根据合并模式选择处理方式
	if dc.Markdown.IsMerge {
		return dc.processMergeMode(processor)
	}
	return dc.processNonMergeMode(processor)
}

// processSingleFile 处理单个文件
func (dc *DocsConfig) processSingleFile(processor *render.FileProcessor, file os.DirEntry) error {
	if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createRenderer()
	if err != nil {
		return err
	}

	// 设置文件处理器
	processor.InputFile = file.Name()
	processor.OutputFile = render.ChangeFileExtFromYamlToMd(file.Name())

	// 如果是 GithubMarkdownRender，设置处理器
	if gh, ok := renderer.(*gh.GithubMarkdownRender); ok {
		gh.SetProcessor(processor)
	}

	// 处理文件
	return render.ProcessFile(processor, renderer)
}

// processNonMergeMode 处理非合并模式
func (dc *DocsConfig) processNonMergeMode(processor *render.FileProcessor) error {
	// 确保输入目录存在
	if _, err := os.Stat(processor.Src); os.IsNotExist(err) {
		return err
	}

	// 确保输出目录存在
	if err := os.MkdirAll(processor.TargetDir, 0o755); err != nil {
		return err
	}

	files, err := os.ReadDir(processor.Src)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !slices.Contains(processor.Exclude, file.Name()) {
			if err := dc.processSingleFile(processor, file); err != nil {
				return err
			}
		}
	}

	return nil
}

// processMergeMode 处理合并模式
func (dc *DocsConfig) processMergeMode(processor *render.FileProcessor) error {
	renderer, err := dc.createRenderer()
	if err != nil {
		return err
	}

	// 如果是 GithubMarkdownRender，设置处理器
	if gh, ok := renderer.(*gh.GithubMarkdownRender); ok {
		gh.SetProcessor(processor)
	}

	return render.ProcessFile(processor, renderer)
}
