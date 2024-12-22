package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

var (
	ghFiles    []string
	folderName string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gh-merge",
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
	rootCmd.Flags().StringVar(&folderName, "folder", "./", "配置文件所在文件夹")
	rootCmd.Flags().StringSliceVar(&ghFiles, "gf", []string{}, "要合并的gh.yml文件列表")
}

func runMerge(cmd *cobra.Command, args []string) {
	var cr gh.ConfigRepos

	// 读取文件夹中的文件
	files, err := os.ReadDir(folderName)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && slices.Contains(ghFiles, file.Name()) {
			fx, err := os.ReadFile(filepath.Join(folderName, file.Name()))
			if err != nil {
				log.Fatalf("error reading file: %v", err)
			}
			// ft, _ := parser.NewParser[gh.ConfigRepo](fx).ParseFlatten()
			ft := gh.NewConfigRepos(fx)
			cr = append(cr, ft.WithTag(strings.TrimSuffix(file.Name(), ".yml")).WithType()...)
		}
	}

	// 定义输出文件路径
	outputPath := "gh.yml"

	// 将合并后的数据写入 YAML 文件
	if err := WriteYAMLToFile(cr, outputPath); err != nil {
		log.Fatalf("error writing to YAML file: %v", err)
	}

	fmt.Printf("Merged YAML file created: %s\n", outputPath)
}

type Gh []string

// WriteYAMLToFile 将 YAML 数据写入文件
func WriteYAMLToFile(data gh.ConfigRepos, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(data); err != nil {
		return err
	}
	return nil
}
