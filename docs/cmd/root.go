package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

// TaskConfig 定义任务结构
type TaskConfig struct {
	SrcDir     string   `yaml:"srcDir"`     // 源目录
	Cmd        string   `yaml:"cmd"`        // 命令类型
	TargetFile string   `yaml:"targetFile"` // 输出文件名
	MoveTo     string   `yaml:"moveTo"`     // 移动目标目录
	Exclude    []string `yaml:"exclude"`    // 排除的目录
	X          []string `yaml:"x"`          // 额外参数
	IsMerge    bool     `yaml:"isMerge"`    // 是否合并
}

// Config 定义配置结构
type Config struct {
	SrcDir     string       `yaml:"srcDir"`     // 源目录
	TargetDir  string       `yaml:"targetDir"`  // 目标目录
	TargetFile string       `yaml:"targetFile"` // 输出文件名
	Tasks      []TaskConfig `yaml:"tasks"`      // 任务列表
	IsRawLoad  bool         `yaml:"isRawLoad"`  // 是否直接加载
}

// processRawLoad 处理 raw-loader 模式
func processRawLoad(srcDir, targetDir string, config Config, task TaskConfig) error {
	inputDir := filepath.Join(srcDir, task.SrcDir)
	outputFile := task.TargetFile
	if outputFile == "" {
		outputFile = config.TargetFile
	}

	// 创建渲染器
	renderer, err := createRenderer(task.Cmd)
	if err != nil {
		return err
	}

	// 渲染内容
	content, err := renderer.Render([]byte(inputDir))
	if err != nil {
		return err
	}

	// 写入输出文件
	outputPath := filepath.Join(targetDir, outputFile)
	return os.WriteFile(outputPath, []byte(content), 0o644)
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

	// 处理文件
	return render.ProcessFile(processor, renderer)
}

// processNonMergeMode 处理非合并模式
func processNonMergeMode(processor *render.FileProcessor, cmd string) error {
	// 确保输入目录存在
	if _, err := os.Stat(processor.InputDir); os.IsNotExist(err) {
		return err
	}

	// 确保输出目录存在
	if err := os.MkdirAll(processor.OutputDir, 0o755); err != nil {
		return err
	}

	files, err := os.ReadDir(processor.InputDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := processSingleFile(processor, file, cmd); err != nil {
			return err
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

	return render.ProcessFile(processor, renderer)
}

// processTask 处理单个任务
func processTask(srcDir, targetDir string, config Config, task TaskConfig) error {
	// 如果配置了 isRawLoad，直接处理整个目录
	if config.IsRawLoad {
		return processRawLoad(srcDir, targetDir, config, task)
	}

	// 创建文件处理器
	processor := &render.FileProcessor{
		InputDir:  filepath.Join(srcDir, task.SrcDir),
		OutputDir: filepath.Join(targetDir, task.SrcDir),
		IsMerge:   task.IsMerge,
	}

	// 根据合并模式选择处理方式
	if task.IsMerge {
		return processMergeMode(processor, task.Cmd)
	}
	return processNonMergeMode(processor, task.Cmd)
}

// processConfig 处理单个配置
func processConfig(config Config) error {
	// 获取绝对路径
	srcDir, err := getAbsPath(config.SrcDir)
	if err != nil {
		return err
	}
	targetDir, err := getAbsPath(config.TargetDir)
	if err != nil {
		return err
	}

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	// 处理每个任务
	for _, task := range config.Tasks {
		if err := processTask(srcDir, targetDir, config, task); err != nil {
			return err
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
