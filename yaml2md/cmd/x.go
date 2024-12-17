package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg"
	"github.com/xbpk3t/docs-alfred/service/gh"

	"github.com/spf13/cobra"
)

var xCmd = &cobra.Command{
	Use:   "x",
	Short: "处理 interview 配置文件",
	Run:   runX,
}

func init() {
	rootCmd.AddCommand(xCmd)
	xCmd.Flags().StringVarP(&folder, "folder", "f", "", "配置文件所在文件夹")
}

var folder string

type Y struct {
	File string   `yaml:"file"`
	Repo []string `yaml:"repo"`
}

type Y2M []Y

// runX 主执行函数
func runX(cmd *cobra.Command, args []string) {
	f, _ := os.ReadFile(cfgFile)

	iv, err := NewConfigX(f)
	if err != nil {
		slog.Error("加载配置失败", slog.String("error", err.Error()))
		return
	}

	content, err := processFiles(iv)
	if err != nil {
		slog.Error("处理文件失败", slog.String("error", err.Error()))
		return
	}

	if err := writeOutput(content); err != nil {
		slog.Error("写入输出失败", slog.String("error", err.Error()))
		return
	}

	slog.Info("Markdown输出已写入", slog.String("File", pkg.ChangeFileExtFromYamlToMd(cfgFile)))
}

// NewConfigX 创建新的配置
func NewConfigX(f []byte) (y2m Y2M, err error) {
	return pkg.Parse[Y](f)
}

// processFiles 处理所有文件并生成内容
func processFiles(iv Y2M) (string, error) {
	renderer := &pkg.MarkdownRenderer{}

	for _, x := range iv {
		repos, err := processFile(x, iv) // 传入完整的 iv
		if err != nil {
			return "", fmt.Errorf("处理文件 %s 失败: %w", x.File, err)
		}

		renderer.RenderHeader(2, x.File)
		// 直接使用 gh.RenderRepositoriesAsMarkdownTable
		renderer.Write(gh.RenderRepositoriesAsMarkdownTable(repos))
	}

	return renderer.String(), nil
}

// processFile 处理单个文件
func processFile(x Y, iv Y2M) (gh.Repos, error) { // 修改参数类型为 Y
	fp := getFilePath(x.File)

	// 使用 MergeFiles 处理文件
	err := pkg.MergeFiles[gh.ConfigRepos](
		filepath.Dir(fp),
		[]string{filepath.Base(fp)},
		"temp.yml",
	)
	if err != nil {
		return nil, fmt.Errorf("合并文件失败: %w", err)
	}

	// 读取处理后的文件
	f, err := os.ReadFile("temp.yml")
	if err != nil {
		return nil, fmt.Errorf("读取临时文件失败: %w", err)
	}

	df, err := gh.ParseConfig(f)
	if err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return MatchRepos(df.ToRepos(), iv), nil
}

// getFilePath 获取文件路径
func getFilePath(fileName string) string {
	if folder == "" {
		return fileName + ".yml"
	}
	return filepath.Join(folder, fileName+".yml")
}

// writeOutput 写入输出文件
func writeOutput(content string) error {
	targetFile := pkg.ChangeFileExtFromYamlToMd(cfgFile)
	return os.WriteFile(targetFile, []byte(content), os.ModePerm)
}

// MatchRepos 匹配仓库
func MatchRepos(repos gh.Repos, y2m Y2M) gh.Repos {
	matchedRepos := make(gh.Repos, 0)
	repoURLMap := make(map[string]gh.Repository)

	for _, repo := range repos {
		repoURLMap[repo.URL] = repo
	}

	for _, item := range y2m {
		for _, repoURL := range item.Repo {
			if matchedRepo, exists := repoURLMap[repoURL]; exists {
				matchedRepos = append(matchedRepos, matchedRepo)
			}
		}
	}

	return matchedRepos
}
