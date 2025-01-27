package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gookit/goutil/fsutil"

	"github.com/xbpk3t/docs-alfred/service/diary"
	"github.com/xbpk3t/docs-alfred/service/goods"
	taskService "github.com/xbpk3t/docs-alfred/service/task"
	"github.com/xbpk3t/docs-alfred/service/works"
	"github.com/xbpk3t/docs-alfred/service/ws"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/gh"
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
func NewDocsConfig(src, cmd string) *DocsConfig {
	return &DocsConfig{
		Src: src,
		Cmd: cmd,
	}
}

// Process 处理配置
func (dc *DocsConfig) Process() error {
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

	// 处理 Markdown 输出
	if dc.Markdown != nil {
		if err := dc.parseMarkdown(); err != nil {
			return fmt.Errorf("parse Markdown error: %w", err)
		}
	}

	// 处理 JSON 输出
	if dc.JSON != nil {
		if err := dc.parseJSON(); err != nil {
			return fmt.Errorf("parse JSON error: %w", err)
		}
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
		IsMerge:    dc.IsDir, // 根据路径类型决定是否合并
	}

	// 如果是单个文件，设置文件信息
	if !dc.IsDir {
		processor.InputFile = filepath.Base(dc.Src)
		processor.Src = filepath.Dir(dc.Src)
	}

	// 读取文件
	data, err := processor.ReadInput()
	if err != nil {
		return fmt.Errorf("read input error: %w", err)
	}

	// 创建渲染器并渲染
	renderer, err := dc.createJSONRenderer()
	if err != nil {
		return fmt.Errorf("create json renderer error: %w", err)
	}
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

	switch dc.IsDir {
	case true:
		return dc.parseDir(processor)
	case false:
		return dc.processSingleFile(processor)
	default:
		return fmt.Errorf("%s neither dir nor file", dc.Src)
	}
}

func (dc *DocsConfig) parseDir(processor *render.FileProcessor) error {
	// 处理目录
	if dc.Markdown.IsMerge {
		return dc.processMergeMode(processor)
	}
	return dc.processNonMergeMode(processor)
}

// processSingleFile 处理单个文件
func (dc *DocsConfig) processSingleFile(processor *render.FileProcessor) error {
	fn := processor.Src

	if fsutil.IsDir(fn) || filepath.Ext(fn) != ".yml" {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createMarkdownRenderer()
	if err != nil {
		return err
	}

	// 设置文件处理器
	// processor.InputFile = file.Name()
	// processor.OutputFile = render.ChangeFileExtFromYamlToMd(fn)

	// 如果是 GithubMarkdownRender，设置处理器
	if gr, ok := renderer.(*gh.GithubMarkdownRender); ok {
		gr.SetProcessor(processor)
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
			if err := dc.processSingleFile(processor); err != nil {
				return err
			}
		}
	}

	return nil
}

// processMergeMode 处理合并模式
func (dc *DocsConfig) processMergeMode(processor *render.FileProcessor) error {
	renderer, err := dc.createMarkdownRenderer()
	if err != nil {
		return err
	}

	// 如果是 GithubMarkdownRender，设置处理器
	if ghr, ok := renderer.(*gh.GithubMarkdownRender); ok {
		ghr.SetProcessor(processor)
	}

	return render.ProcessFile(processor, renderer)
}

func (dc *DocsConfig) createJSONRenderer() (render.Renderer, error) {
	// 如果配置了JSON输出，使用JSON渲染器
	if dc.JSON != nil {
		return render.NewJSONRenderer(dc.Cmd, true), nil
	}
	return nil, fmt.Errorf("please add JSON for entity: %s", dc.Cmd)
}

// createMarkdownRenderer 创建渲染器
func (dc *DocsConfig) createMarkdownRenderer() (render.Renderer, error) {
	if dc.Markdown != nil {
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
			return nil, fmt.Errorf("markdown Render fail: unknown command: %s", dc.Cmd)
		}
	}

	return nil, fmt.Errorf("please add markdown for entity: %s", dc.Cmd)
}

// parseJSONFile 处理单个JSON文件
//func (dc *DocsConfig) parseJSONFile() error {
//	if dc.JSON == nil {
//		return nil
//	}
//
//	// 读取单个文件
//	data, err := os.ReadFile(dc.Src)
//	if err != nil {
//		return fmt.Errorf("read file error: %w", err)
//	}
//
//	// 创建文件处理器
//	processor := &render.FileProcessor{
//		Src:        filepath.Dir(dc.Src),
//		InputFile:  filepath.Base(dc.Src),
//		TargetDir:  filepath.Dir(dc.JSON.Dst),
//		OutputFile: filepath.Base(dc.JSON.Dst),
//		IsMerge:    false, // 单文件模式不需要合并
//	}
//
//	// 创建渲染器并渲染
//	renderer := render.NewJSONRenderer(dc.Cmd, true)
//	content, err := renderer.Render(data)
//	if err != nil {
//		return fmt.Errorf("render error: %w", err)
//	}
//
//	// 写入文件
//	if err := processor.WriteOutput(content); err != nil {
//		return fmt.Errorf("write output error: %w", err)
//	}
//
//	return nil
//}
