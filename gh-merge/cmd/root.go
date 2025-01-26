package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/parser"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"

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
	RunE:  runMerge,
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

func runMerge(cmd *cobra.Command, args []string) error {
	var cr gh.ConfigRepos

	// 读取文件夹中的文件
	files, err := os.ReadDir(folderName)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}

	// 用于去重的map
	seen := make(map[string]struct{})

	for _, file := range files {
		if !file.IsDir() && slices.Contains(ghFiles, file.Name()) {
			fx, err := os.ReadFile(filepath.Join(folderName, file.Name()))
			if err != nil {
				log.Fatalf("error reading file: %v", err)
			}

			// 解析并处理仓库
			rc, err := parser.NewParser[gh.ConfigRepos](fx).ParseSingle()
			if err != nil {
				return errcode.WithError(errcode.ErrParseConfig, fmt.Errorf("%s: %w", file.Name(), err))
			}

			repos := rc.WithType().WithTag(strings.TrimSuffix(file.Name(), ".yml")).ToRepos()
			if repos == nil {
				return errcode.WithError(errcode.ErrInvalidConfig, err)
			}

			// 过滤掉作为子仓库的仓库
			var mainRepos gh.Repos
			for _, repo := range repos {
				if !repo.IsSubOrDepOrRelRepo() {
					if _, exists := seen[repo.URL]; !exists {
						seen[repo.URL] = struct{}{}
						mainRepos = append(mainRepos, repo)
					}
				}
			}

			cr = append(cr, gh.ConfigRepo{
				Type:  strings.TrimSuffix(file.Name(), ".yml"),
				Repos: mainRepos,
			})
		}
	}

	// 定义输出文件路径
	outputPath := "gh.yml"

	// 将合并后的数据写入 Markdown 文件
	if err := WriteYAMLToFile(cr, outputPath); err != nil {
		log.Fatalf("error writing to Markdown file: %v", err)
	}

	fmt.Printf("Merged Markdown file created: %s\n", outputPath)
	return nil
}

type Gh []string

// WriteYAMLToFile 将 Markdown 数据写入文件
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
