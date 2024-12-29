package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/ws"

	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/works"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:       "yaml2md",
	ValidArgs: []string{"gh", "works", "ws", "goods", "x"},
	Args:      cobra.OnlyValidArgs,
	Short:     "A brief description of your application",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(ghCmd)
	rootCmd.AddCommand(worksCmd)
	rootCmd.AddCommand(wsCmd)
	rootCmd.AddCommand(goodsCmd)
	rootCmd.AddCommand(xCmd)
	rootCmd.AddCommand(diaryCmd)
	rootCmd.AddCommand(taskCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "src/data/qs.yml", "config file (default is src/data/qs.yml)")
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("src/data/")
		viper.SetConfigName("qs")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Convert GitHub repos yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := gh.NewGhRenderer()
		return ProcessFile(cfgFile, renderer)
	},
}

var worksCmd = &cobra.Command{
	Use:   "works",
	Short: "Convert works yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := works.NewWorkRenderer()
		return ProcessFile(cfgFile, renderer)
	},
}

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Convert website links yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := ws.NewWebStackRenderer()
		return ProcessFile(cfgFile, renderer)
	},
}

// cmd/goods.go
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "Convert goods yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := goods.NewGoodsRenderer()
		return ProcessFile(cfgFile, renderer)
	},
}

var xCmd = &cobra.Command{
	Use:   "x",
	Short: "处理 interview 配置文件",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &gh.XRenderer{}
		return ProcessFile(cfgFile, renderer)
	},
}

// diaryCmd represents the diary command
var diaryCmd = &cobra.Command{
	Use:   "diary",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("diary called")
	},
}

// taskCmd represents the task command
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("task called")
	},
}

// ChangeFileExtFromYamlToMd 将yaml文件扩展名改为md
func ChangeFileExtFromYamlToMd(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext) + ".md"
}

// ProcessFile 处理文件转换
func ProcessFile(inputFile string, renderer render.MarkdownRender) error {
	fp := &render.FileProcessor{
		InputFile:  inputFile,
		OutputFile: ChangeFileExtFromYamlToMd(inputFile),
	}

	// 读取文件
	data, err := fp.ReadInput()
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 渲染内容
	content, err := renderer.Render(data)
	if err != nil {
		return fmt.Errorf("渲染失败: %w", err)
	}

	// 写入文件
	if err := fp.WriteOutput([]byte(content)); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
