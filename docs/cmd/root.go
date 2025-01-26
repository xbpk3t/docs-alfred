package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/diary"
	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	taskService "github.com/xbpk3t/docs-alfred/service/task"
	"github.com/xbpk3t/docs-alfred/service/works"
	"github.com/xbpk3t/docs-alfred/service/ws"

	"github.com/spf13/cobra"
)

var cfgFile string

// Config 定义配置结构
type Config struct {
	Markdown *YAML  `yaml:"markdown"` // Using pointer to allow nil checks
	JSON     *JSON  `yaml:"json"`     // Using pointer to allow nil checks
	SrcDir   string `yaml:"srcDir"`   // 源目录
	Cmd      string `yaml:"cmd"`      // 命令类型
}

type YAML struct {
	YamlDst         string   `yaml:"yamlDst"`
	MergeOutputFile string   `yaml:"mergeOutputFile"` // 合并后的输出文件名
	Exclude         []string `yaml:"exclude"`
	IsMerge         bool     `yaml:"isMerge"`
	IsRawLoad       bool     `yaml:"isRawLoad"` // 是否直接加载
	IsExpand        bool     `yaml:"isExpand"`  // 在docusaurus中是否展开
}

type JSON struct {
	JSONDst string `yaml:"jsonDst"`
}

// processSingleFile 处理单个文件
func processSingleFile(processor *render.FileProcessor, file os.DirEntry, cmd string) error {
	if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
		return nil
	}

	// 创建渲染器
	renderer, err := createRenderer(cmd)
	if err != nil {
		return err
	}

	// 设置文件处理器
	processor.InputFile = file.Name()
	processor.OutputFile = render.ChangeFileExtFromYamlToMd(file.Name())

	// 如果是 GhRenderer，设置处理器
	if gh, ok := renderer.(*gh.GhRenderer); ok {
		gh.SetProcessor(processor)
	}

	// 处理文件
	return render.ProcessFile(processor, renderer)
}

// processNonMergeMode 处理非合并模式
func processNonMergeMode(processor *render.FileProcessor, cmd string) error {
	// 确保输入目录存在
	if _, err := os.Stat(processor.SrcDir); os.IsNotExist(err) {
		return err
	}

	// 确保输出目录存在
	if err := os.MkdirAll(processor.TargetDir, 0o755); err != nil {
		return err
	}

	files, err := os.ReadDir(processor.SrcDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !slices.Contains(processor.Exclude, file.Name()) {
			if err := processSingleFile(processor, file, cmd); err != nil {
				return err
			}
		}
	}

	return nil
}

// processMergeMode 处理合并模式
func processMergeMode(processor *render.FileProcessor, cmd string) error {
	renderer, err := createRenderer(cmd)
	if err != nil {
		return err
	}

	// 如果是 GhRenderer，设置处理器
	if gh, ok := renderer.(*gh.GhRenderer); ok {
		gh.SetProcessor(processor)
	}

	return render.ProcessFile(processor, renderer)
}

// parseMarkdown handles YAML configuration processing
func parseMarkdown(config Config) error {
	if config.Markdown == nil {
		return nil
	}

	// 获取绝对路径
	srcDir, err := getAbsPath(config.SrcDir)
	if err != nil {
		return err
	}

	// 获取目标路径
	targetDir := config.Markdown.YamlDst
	if targetDir == "" {
		targetDir = "docs" // 默认输出到docs目录
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		SrcDir:    srcDir,
		TargetDir: targetDir,
		IsMerge:   config.Markdown.IsMerge,
		Exclude:   config.Markdown.Exclude,
	}

	// 根据合并模式选择处理方式
	if config.Markdown.IsMerge {
		return processMergeMode(processor, config.Cmd)
	}
	return processNonMergeMode(processor, config.Cmd)
}

// parseJSON handles JSON configuration processing
func parseJSON(config Config) error {
	if config.JSON == nil {
		return nil
	}

	// 获取绝对路径
	srcDir, err := getAbsPath(config.SrcDir)
	if err != nil {
		return err
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		SrcDir:     srcDir,
		TargetDir:  filepath.Dir(config.JSON.JSONDst),
		OutputFile: filepath.Base(config.JSON.JSONDst),
	}

	// 创建渲染器
	renderer, err := createRenderer(config.Cmd)
	if err != nil {
		return err
	}

	// 处理文件
	return render.ProcessFile(processor, renderer)
}

// processConfig 处理单个配置
func processConfig(config Config) error {
	if config.Markdown != nil {
		if err := parseMarkdown(config); err != nil {
			return fmt.Errorf("parse Markdown error: %w", err)
		}
	}

	if config.JSON != nil {
		if err := parseJSON(config); err != nil {
			return fmt.Errorf("parse JSON error: %w", err)
		}
	}

	return nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "docs-alfred",
	Short: "根据 docs.yml 生成文档",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 读取配置文件
		configData, err := os.ReadFile(cfgFile)
		if err != nil {
			return err
		}

		// 解析配置
		configs, err := parser.NewParser[[]Config](configData).ParseSingle()
		if err != nil {
			return err
		}

		// 处理每个配置
		for _, config := range configs {
			if err := processConfig(config); err != nil {
				return err
			}
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "docs.yml", "配置文件路径")
}

// getAbsPath 获取绝对路径
func getAbsPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

// createRenderer 根据命令类型创建渲染器
func createRenderer(cmd string) (render.MarkdownRender, error) {
	switch cmd {
	case "works":
		return works.NewWorkRenderer(), nil
	case "gh":
		return gh.NewGhRenderer(), nil
	case "ws":
		return ws.NewWebStackRenderer(), nil
	case "diary":
		return diary.NewDiaryRenderer(), nil
	case "goods":
		return goods.NewGoodsRenderer(), nil
	case "task":
		return taskService.NewTaskRenderer(), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
}
