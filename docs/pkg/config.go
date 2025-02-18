package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gookit/goutil/fsutil"

	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/works"
	"github.com/xbpk3t/docs-alfred/service/ws"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// DocsConfig 定义配置结构
type DocsConfig struct {
	Markdown *Markdown `yaml:"md"`   // Using pointer to allow nil checks
	JSON     *JSON     `yaml:"json"` // Using pointer to allow nil checks
	Src      string    `yaml:"src"`  // 源路径
	Cmd      string    `yaml:"cmd"`  // 命令类型
	IsDir    bool      `yaml:"-"`    // 是否为文件夹，根据src自动判断
}

type Markdown struct {
	Dst             string `yaml:"dst"`             // 输出目录
	MergeOutputFile string `yaml:"mergeOutputFile"` // 合并后的输出文件名

	// 内部使用的字段
	currentFile string   // 当前处理的文件名
	Exclude     []string `yaml:"exclude"`   // 排除的文件
	IsMerge     bool     `yaml:"isMerge"`   // 是否合并模式
	IsRawLoad   bool     `yaml:"isRawLoad"` // 是否直接加载
	IsExpand    bool     `yaml:"isExpand"`  // 在docusaurus中是否展开
}

// GetCurrentFileName 获取当前处理的文件名
func (m *Markdown) GetCurrentFileName() string {
	return m.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (m *Markdown) SetCurrentFile(filename string) {
	m.currentFile = filename
}

// GetInputPath 获取输入文件完整路径
func (m *Markdown) GetInputPath(src string) string {
	if fsutil.IsFile(src) {
		return src
	}
	return filepath.Join(src, m.MergeOutputFile)
}

// GetOutputPath 获取输出文件完整路径
func (m *Markdown) GetOutputPath(filename string) string {
	if m.MergeOutputFile != "" {
		return filepath.Join(m.Dst, m.MergeOutputFile)
	}
	return filepath.Join(m.Dst, filename)
}

// ReadInput 读取输入
func (m *Markdown) ReadInput(src string, isDir bool) ([]byte, error) {
	if m.IsMerge && isDir {
		return m.readAndMergeFiles(src)
	}
	return m.readSingleFile(src)
}

// readSingleFile 读取单个文件
func (m *Markdown) readSingleFile(src string) ([]byte, error) {
	// 检查src是否是目录
	fileInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("stat path error: %w", err)
	}

	var inputPath string
	if fileInfo.IsDir() {
		// 如果是目录，读取第一个yml文件
		files, err := os.ReadDir(src)
		if err != nil {
			return nil, fmt.Errorf("read dir error: %w", err)
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
				continue
			}
			inputPath = filepath.Join(src, file.Name())
			m.SetCurrentFile(file.Name())
			break
		}

		if inputPath == "" {
			return nil, fmt.Errorf("no yml file found in directory: %s", src)
		}
	} else {
		// 如果是文件，直接使用
		inputPath = src
		m.SetCurrentFile(filepath.Base(src))
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}
	return data, nil
}

// readAndMergeFiles 读取并合并文件
func (m *Markdown) readAndMergeFiles(src string) ([]byte, error) {
	files, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read dir error: %w", err)
	}

	var mergedData []byte
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" || slices.Contains(m.Exclude, file.Name()) {
			continue
		}

		m.SetCurrentFile(file.Name())
		data, err := os.ReadFile(filepath.Join(src, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("read file error: %w", err)
		}
		mergedData = append(mergedData, data...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData, nil
}

// WriteOutput 写入输出
func (m *Markdown) WriteOutput(content string, filename string) error {
	// 确保输出目录存在
	if err := os.MkdirAll(m.Dst, 0o755); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(m.Dst, filename)
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// ProcessFile 核心方法：处理单个文件
func (m *Markdown) ProcessFile(src string, renderer render.Renderer) error {
	// 读取文件
	data, err := m.ReadInput(src, fsutil.IsDir(src))
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// 确定输出文件名
	outputFilename := m.MergeOutputFile
	if outputFilename == "" {
		if fsutil.IsDir(src) {
			outputFilename = filepath.Base(src) + ".md"
		} else {
			outputFilename = strings.TrimSuffix(filepath.Base(src), filepath.Ext(filepath.Base(src))) + ".md"
		}
	}

	// 写入文件
	if err := m.WriteOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

type JSON struct {
	Dst             string `yaml:"dst"`             // 输出目录
	MergeOutputFile string `yaml:"mergeOutputFile"` // 合并后的输出文件名

	// 内部使用的字段
	currentFile string   // 当前处理的文件名
	Exclude     []string `yaml:"exclude"` // 排除的文件
	IsMerge     bool     `yaml:"isMerge"` // 是否合并模式
}

// GetCurrentFileName 获取当前处理的文件名
func (j *JSON) GetCurrentFileName() string {
	return j.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (j *JSON) SetCurrentFile(filename string) {
	j.currentFile = filename
}

// ReadInput 读取输入
func (j *JSON) ReadInput(src string, isDir bool) ([]byte, error) {
	if isDir {
		return j.readAndMergeFiles(src)
	}
	return j.readSingleFile(src)
}

// readSingleFile 读取单个文件
func (j *JSON) readSingleFile(src string) ([]byte, error) {
	// 检查src是否是目录
	fileInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("stat path error: %w", err)
	}

	var inputPath string
	if fileInfo.IsDir() {
		// 如果是目录，读取第一个yml文件
		files, err := os.ReadDir(src)
		if err != nil {
			return nil, fmt.Errorf("read dir error: %w", err)
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
				continue
			}
			inputPath = filepath.Join(src, file.Name())
			j.SetCurrentFile(file.Name())
			break
		}

		if inputPath == "" {
			return nil, fmt.Errorf("no yml file found in directory: %s", src)
		}
	} else {
		// 如果是文件，直接使用
		inputPath = src
		j.SetCurrentFile(filepath.Base(src))
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}
	return data, nil
}

// readAndMergeFiles 读取并合并文件
func (j *JSON) readAndMergeFiles(src string) ([]byte, error) {
	files, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read dir error: %w", err)
	}

	var mergedData []byte
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" || slices.Contains(j.Exclude, file.Name()) {
			continue
		}

		j.SetCurrentFile(file.Name())
		data, err := os.ReadFile(filepath.Join(src, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("read file error: %w", err)
		}
		mergedData = append(mergedData, data...)
		mergedData = append(mergedData, '\n')
	}

	return mergedData, nil
}

// WriteOutput 写入输出
func (j *JSON) WriteOutput(content string, filename string) error {
	// 确保输出目录存在
	outputDir := filepath.Dir(j.Dst)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	if err := os.WriteFile(j.Dst, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// ProcessFile 核心方法：处理单个文件
func (j *JSON) ProcessFile(src string, renderer render.Renderer) error {
	// 读取文件
	data, err := j.ReadInput(src, fsutil.IsDir(src))
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// 确定输出文件名
	outputFilename := j.MergeOutputFile
	if outputFilename == "" {
		if fsutil.IsDir(src) {
			outputFilename = filepath.Base(src) + ".json"
		} else {
			outputFilename = strings.TrimSuffix(filepath.Base(src), filepath.Ext(filepath.Base(src))) + ".json"
		}
	}

	// 写入文件
	if err := j.WriteOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
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

// parseMarkdown 处理Markdown配置
func (dc *DocsConfig) parseMarkdown() error {
	if dc.Markdown == nil {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createMarkdownRenderer()
	if err != nil {
		return fmt.Errorf("create renderer error: %w", err)
	}

	// 如果是 GithubMarkdownRender，设置当前文件
	if gr, ok := renderer.(*gh.GithubMarkdownRender); ok {
		gr.SetCurrentFile(filepath.Base(dc.Src))
	}

	// 处理文件
	return dc.Markdown.ProcessFile(dc.Src, renderer)
}

// parseJSON 处理JSON配置
func (dc *DocsConfig) parseJSON() error {
	if dc.JSON == nil {
		return nil
	}

	// 创建渲染器
	renderer, err := dc.createJSONRenderer()
	if err != nil {
		return fmt.Errorf("create renderer error: %w", err)
	}

	// 处理文件
	return dc.JSON.ProcessFile(dc.Src, renderer)
}

func (dc *DocsConfig) createJSONRenderer() (render.Renderer, error) {
	// 如果配置了JSON输出，使用JSON渲染器
	if dc.JSON != nil {
		renderer := render.NewJSONRenderer(dc.Cmd, true)

		// 根据不同的命令类型设置解析模式
		switch dc.Cmd {
		case "goods":
			renderer.WithParseMode(render.ParseFlatten)
		case "works":
			renderer.WithParseMode(render.ParseMulti)
		default:
			renderer.WithParseMode(render.ParseSingle)
		}

		return renderer, nil
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
		case "goods":
			return goods.NewGoodsMarkdownRenderer(), nil
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
