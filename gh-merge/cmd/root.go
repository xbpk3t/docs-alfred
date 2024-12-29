package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

var (
	folderName = "."
	ghFiles    = []string{"gh.yml", "gh-sub.yml", "gh-rel.yml"}
)

var rootCmd = &cobra.Command{
	Use:   "gh-merge",
	Short: "Merge multiple gh.yml files",
	Run:   runMerge,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// validateInput 验证输入参数
func validateInput() error {
	if folderName == "" {
		return errcode.ErrInvalidInput
	}
	return nil
}

// writeResult 写入结果到文件
func writeResult(repos gh.ConfigRepos) error {
	// 定义输出文件路径
	outputPath := "gh.yml"

	// 将合并后的数据写入 YAML 文件
	data, err := yaml.Marshal(repos)
	if err != nil {
		return errcode.WithError(errcode.ErrEncodeYAML, err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return errcode.WithError(errcode.ErrWriteFile, err)
	}

	fmt.Printf("Merged YAML file created: %s\n", outputPath)
	return nil
}

func runMerge(cmd *cobra.Command, args []string) {
	if err := executeMerge(cmd, args); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func executeMerge(cmd *cobra.Command, args []string) error {
	// 验证输入
	if err := validateInput(); err != nil {
		return err
	}

	// 读取文件夹中的文件
	files, err := os.ReadDir(folderName)
	if err != nil {
		return errcode.WithError(errcode.ErrListDir, err)
	}

	var cr gh.ConfigRepos
	seen := make(map[string]struct{})

	for _, file := range files {
		if !file.IsDir() && slices.Contains(ghFiles, file.Name()) {
			fx, err := os.ReadFile(filepath.Join(folderName, file.Name()))
			if err != nil {
				return errcode.WithError(errcode.ErrReadFile, err)
			}

			// 解析并处理仓库
			rc := gh.NewConfigRepos(fx)
			if rc == nil {
				return errcode.ErrParseConfig
			}

			repos := rc.WithType().WithTag(strings.TrimSuffix(file.Name(), ".yml")).ToRepos()
			if repos == nil {
				return errcode.ErrInvalidConfig
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

	return writeResult(cr)
}
