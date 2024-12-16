package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/utils"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// xCmd represents the x command
var xCmd = &cobra.Command{
	Use:   "x",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}
		iv := NewConfigX(f)

		var res strings.Builder

		for _, x := range iv.X {
			var fp string
			if folder == "" {
				fp = x.File + ".yml"
			} else {
				fp = fmt.Sprintf("%s/%s.yml", folder, x.File)
			}
			f, err := os.ReadFile(fp)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			df := gh.NewConfigRepos(f)

			res.WriteString(fmt.Sprintf("## %s\n", x.File))

			repos := gh.RenderRepos(MatchRepos(df.ToRepos(), iv))
			res.WriteString(repos.String())
		}

		targetFile := utils.ChangeFileExtFromYamlToMd(cfgFile)
		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

func init() {
	rootCmd.AddCommand(xCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// xCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// xCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	xCmd.Flags().StringVarP(&folder, "folder", "f", "", "")
}

var folder string

type Y2M struct {
	X []struct {
		File string   `yaml:"file"`
		Repo []string `yaml:"repo"`
	} `yaml:"interview"`
}

func NewConfigX(f []byte) (y2m Y2M) {
	err := yaml.Unmarshal(f, &y2m)
	if err != nil {
		panic(err)
	}

	return y2m
}

// MatchRepos 函数接受 Repos 和 Y2M 类型的参数，返回匹配的 Repos 切片
func MatchRepos(repos gh.Repos, y2m Y2M) gh.Repos {
	matchedRepos := make(gh.Repos, 0)
	repoURLMap := make(map[string]gh.Repository)

	// 将所有 Repository 的 URL 作为 key 存储在 map 中
	for _, repo := range repos {
		repoURLMap[repo.URL] = repo
	}

	// 遍历 Y2M 结构体中的 Repo 列表
	for _, item := range y2m.X {
		for _, repoURL := range item.Repo {
			// 如果 URL 在 map 中，添加到结果切片中
			if matchedRepo, exists := repoURLMap[repoURL]; exists {
				matchedRepos = append(matchedRepos, matchedRepo)
			}
		}
	}

	return matchedRepos
}
