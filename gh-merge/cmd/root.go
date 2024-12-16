package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/gh"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
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
	cr, err := mergeConfigFiles()
	if err != nil {
		log.Fatalf("合并配置文件失败: %v", err)
	}

	if err := writeToYAML(cr, "gh.yml"); err != nil {
		log.Fatalf("写入YAML文件失败: %v", err)
	}

	fmt.Printf("成功创建合并后的YAML文件: gh.yml\n")
}

// mergeConfigFiles 合并配置文件
func mergeConfigFiles() (gh.ConfigRepos, error) {
	var cr gh.ConfigRepos

	files, err := os.ReadDir(folderName)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	for _, file := range files {
		if shouldProcessFile(file) {
			configs, err := processConfigFile(file)
			if err != nil {
				return nil, err
			}
			cr = append(cr, configs...)
		}
	}

	return cr, nil
}

// shouldProcessFile 判断是否应该处理该文件
func shouldProcessFile(file os.DirEntry) bool {
	return !file.IsDir() && slices.Contains(ghFiles, file.Name())
}

// processConfigFile 处理单个配置文件
func processConfigFile(file os.DirEntry) (gh.ConfigRepos, error) {
	filePath := filepath.Join(folderName, file.Name())
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件 %s 失败: %v", file.Name(), err)
	}

	tag := strings.TrimSuffix(file.Name(), ".yml")
	return gh.NewConfigRepos(content).WithTag(tag), nil
}

// writeToYAML 将数据写入YAML文件
func writeToYAML(data gh.ConfigRepos, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("编码YAML失败: %v", err)
	}
	return nil
}
