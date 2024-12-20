package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/merger"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

var (
	ghFiles    []string
	folderName string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "merge",
	Short: "合并多个 gh.yml 文件",
	Run:   runMerge,
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
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().StringVar(&folderName, "folder", "data/x", "配置文件所在文件夹")
	rootCmd.Flags().StringSliceVar(&ghFiles, "yf", []string{}, "要合并的gh.yml文件列表")
}

func runMerge(cmd *cobra.Command, args []string) {
	err := merger.MergeFiles[gh.ConfigRepos](
		folderName, // 配置文件所在文件夹
		ghFiles,    // 要合并的文件列表
		"gh.yml",   // 输出文件路径
	)
	if err != nil {
		log.Fatalf("合并配置文件失败: %v", err)
	}

	fmt.Printf("成功创建合并后的YAML文件: gh.yml\n")
}
