package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service/ws"
	"os"
	"path/filepath"
	"strings"

	"github.com/xbpk3t/docs-alfred/service/gh"
	"github.com/xbpk3t/docs-alfred/service/goods"
	"github.com/xbpk3t/docs-alfred/service/work"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yaml2md",
	Short: "A brief description of your application",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
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

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yaml2md.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

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
		renderer := &gh.GhRenderer{}
		return ProcessFile(cfgFile, renderer)
	},
}

// cmd/works.go
var worksCmd = &cobra.Command{
	Use:   "works",
	Short: "Convert works yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &work.WorkRenderer{}
		return ProcessFile(cfgFile, renderer)
	},
}

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Convert website links yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &ws.WebStackRenderer{}
		return ProcessFile(cfgFile, renderer)
	},
}

// cmd/goods.go
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "Convert goods yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &goods.GoodsRenderer{}
		return ProcessFile(cfgFile, renderer)
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
