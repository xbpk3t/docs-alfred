package cmd

import (
	"os"
	"path/filepath"
	"strings"

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
	SrcDir    string   `yaml:"srcDir"`    // 源目录
	Cmd       string   `yaml:"cmd"`       // 命令类型
	Fn        string   `yaml:"fn"`        // 输出文件名
	MoveTo    string   `yaml:"moveTo"`    // 移动目标目录
	Exclude   []string `yaml:"exclude"`   // 排除的目录
	X         []string `yaml:"x"`         // 额外参数
	IsMerge   bool     `yaml:"isMerge"`   // 是否合并
	IsRawLoad bool     `yaml:"isRawLoad"` // 是否直接加载
}

// Config 定义配置结构
type Config struct {
	SrcDir    string       `yaml:"srcDir"`    // 源目录
	TargetDir string       `yaml:"targetDir"` // 目标目录
	Fn        string       `yaml:"fn"`        // 文件名
	Tasks     []TaskConfig `yaml:"tasks"`     // 任务列表
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
			// 获取绝对路径
			srcDir, err := getAbsPath(config.SrcDir)
			if err != nil {
				return err
			}
			targetDir, err := getAbsPath(config.TargetDir)
			if err != nil {
				return err
			}

			for _, task := range config.Tasks {
				// 创建文件处理器
				processor := &render.FileProcessor{
					InputDir:   filepath.Join(srcDir, task.SrcDir),
					OutputDir:  filepath.Join(targetDir, task.SrcDir),
					OutputFile: task.Fn,
					IsMerge:    task.IsMerge,
				}

				// 如果不是合并模式，则需要先列出目录下的文件
				if !task.IsMerge {
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
						if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
							continue
						}

						processor.InputFile = file.Name()
						processor.OutputFile = strings.TrimSuffix(file.Name(), ".yml") + ".md"

						// 读取输入文件
						data, err := processor.ReadInput()
						if err != nil {
							return err
						}

						// 渲染内容
						content, err := renderContent(task.Cmd, data)
						if err != nil {
							return err
						}

						// 写入输出
						if err := processor.WriteOutput([]byte(content)); err != nil {
							return err
						}
					}
					continue
				}

				// 合并模式的处理
				data, err := processor.ReadInput()
				if err != nil {
					return err
				}

				content, err := renderContent(task.Cmd, data)
				if err != nil {
					return err
				}

				if err := processor.WriteOutput([]byte(content)); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

// renderContent 根据命令类型渲染内容
func renderContent(cmd string, data []byte) (string, error) {
	var content string
	var err error

	switch cmd {
	case "works":
		renderer := works.NewWorkRenderer()
		content, err = renderer.Render(data)
	case "gh":
		renderer := gh.NewGhRenderer()
		content, err = renderer.Render(data)
	case "ws":
		renderer := ws.NewWebStackRenderer()
		content, err = renderer.Render(data)
	case "diary":
		renderer := diary.NewDiaryRenderer()
		content, err = renderer.Render(data)
	case "goods":
		renderer := goods.NewGoodsRenderer()
		content, err = renderer.Render(data)
	case "cmd":
		renderer := taskService.NewTaskRenderer()
		content, err = renderer.Render(data)
	}

	return content, err
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
