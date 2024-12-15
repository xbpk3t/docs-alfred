package cmd

import (
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/utils"

	"github.com/xbpk3t/docs-alfred/pkg/ws"

	aw "github.com/deanishe/awgo"
	"github.com/xbpk3t/docs-alfred/pkg/gh"

	"github.com/spf13/cobra"
)

const (
	ConfigGithub = "gh.yml"
	RepoDB       = "/repo.db"
)

// fCmd represents the f command
var fCmd = &cobra.Command{
	Use:   "f",
	Short: "A brief description of your command",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// ReadConfig Handle Config Not Exists
		// panic not effect PreRun()
		if !wf.Cache.Exists(cfgFile) {
			// ErrorHandle(errors.New(cfgFile + " not found"))
			ErrorHandle(&utils.DocsAlfredError{Err: utils.ErrConfigNotFound})
		}

		// fetch gh.yml, 再根据内容fetch数据
		data, _ = wf.Cache.Load(cfgFile)

		if !wf.IsRunning(SyncJob) {
			cmd := exec.Command("./exe", SyncJob, fmt.Sprintf("--config=%s", cfgFile))
			slog.Info("sync cmd: ", slog.Any("cmd", cmd.String()))
			if err := wf.RunInBackground(SyncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("f called")
	},
}

var data []byte

func init() {
	rootCmd.AddCommand(fCmd)
	fCmd.AddCommand(ghCmd)
	fCmd.AddCommand(wsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// fCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// fCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

const (
	RepoSearch = "https://github.com/search?q=%s&type=repositories"
	FaStar     = "icons/check.svg"
	FaRepo     = "icons/repo.png"
	FaSearch   = "icons/search.svg"
	FaQs       = "icons/a.svg"
	FaDoc      = "icons/b.svg"
	FaQsAndDoc = "icons/ab.svg"
)

// ghCmd represents the repo command
// Enter            直接打开URL
// CMD + Enter      Markdown View
// Option + Enter   Copy URL
// Shift + Enter    跳转到该repo的doc
var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Searching from starred repositories and my repositories",
	Run: func(cmd *cobra.Command, args []string) {
		// repos := gh.NewRepos()
		// err := repos.ListRepositories(wf.CacheDir() + RepoDB)
		// if err != nil {
		// 	wf.FatalError(err)
		// }

		repos := gh.NewConfigRepos(data).ToRepos()

		if len(args) > 0 && strings.HasPrefix(args[0], "#") {
			tags := repos.ExtractTags()

			// if hit tag
			ptag := strings.TrimPrefix(args[0], "#")
			if slices.Contains(tags, ptag) {
				repos = repos.QueryReposByTag(ptag)
				RenderRepos(repos)
			} else {
				for _, tag := range tags {
					tag = fmt.Sprintf("#%s", tag)
					wf.NewItem(tag).Title(tag).Valid(false).Autocomplete(tag)
				}
				if len(args) > 0 {
					wf.Filter(args[0])
				}
			}

			wf.SendFeedback()
		}

		RenderRepos(repos)

		if len(args) > 0 {
			wf.Filter(args[0])
		}

		wf.NewItem("Search Github").
			Arg(fmt.Sprintf(RepoSearch, strings.Join(args, "+"))).
			Valid(true).
			Icon(&aw.Icon{Value: FaSearch}).
			Title(fmt.Sprintf("Search Github For '%s'", strings.Join(args, " ")))
		// wf.NewItem("Search Gist").
		// 	Arg(fmt.Sprintf(GistSearch, strings.Join(args, "+"))).
		// 	Valid(true).
		// 	Icon(&aw.Icon{Value: FaGists}).
		// 	Title(fmt.Sprintf("Search Gist For '%s'", strings.Join(args, " ")))
		wf.SendFeedback()
	},
}

// wsCmd represents the ws command
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := ws.NewConfigWs(data)
		if err != nil {
			wf.FatalError(err)
		}
		tks := ws.SearchWs(args)

		for _, ws := range tks {
			item := wf.NewItem(ws.Name).Title(ws.Name).Subtitle(ws.Des).Valid(true).Quicklook(ws.URL).Autocomplete(ws.Name).Arg(ws.URL).Icon(&aw.Icon{Value: "icons/check.svg"})

			item.Cmd().Subtitle(fmt.Sprintf("Quicklook: %s", ws.URL)).Arg(ws.Des)
			item.Opt().Subtitle(fmt.Sprintf("Copy URL: %s", ws.URL)).Arg(ws.URL)
		}

		wf.SendFeedback()
	},
}

// // 渲染命令行
// func addMarkdownCmd(cmds gh.Cmd) string {
// 	var builder strings.Builder
// 	for _, cmd := range cmds {
// 		if cmd.K {
// 			// TODO alfred markdown 渲染有问题，无法渲染 ***``***
// 			builder.WriteString(fmt.Sprintf("- ***%s*** %s\n", cmd.C, cmd.X))
// 		} else {
// 			builder.WriteString(fmt.Sprintf("- `%s` %s\n", cmd.C, cmd.X))
// 		}
// 	}
// 	return builder.String()
// }

// func addMarkdownQsFormat(qs gh.Qs) string {
// 	var builder strings.Builder
// 	// builder.WriteString("<dl>")
// 	// for _, q := range qs {
// 	//
// 	// 	builder.WriteString(fmt.Sprintf("- %s \n", q.Q))
// 	// 	builder.WriteString(fmt.Sprintf("\n %s \n", q.X))
// 	// 	// builder.WriteString(fmt.Sprintf("<dt>%s</dt>", q.Q))
// 	// 	// builder.WriteString(fmt.Sprintf("<dd>%s</dd>", q.X))
// 	// }
// 	// // builder.WriteString("</dl>")
// 	//
// 	// return builder.String()
//
// 	for _, q := range qs {
// 		if q.U != "" {
// 			builder.WriteString(fmt.Sprintf("- [%s](%s)\n", q.Q, q.U))
// 		} else {
// 			builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
// 		}
// 	}
// 	return builder.String()
// }

// GetFileNameFromURL 从给定的 URL 中提取并返回文件名。
func GetFileNameFromURL(urlString string) (string, error) {
	// 解析 URL
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %v", err)
	}

	// 获取路径
	urlPath := parsedURL.Path

	// 获取文件名
	fileName := path.Base(urlPath)

	return fileName, nil
}

// func addMarkdownPicFormat(URLs []string) string {
// 	var builder strings.Builder
// 	for _, u := range URLs {
// 		name, _ := GetFileNameFromURL(u)
// 		builder.WriteString(fmt.Sprintf("- [%s](%s)\n", name, u))
// 	}
// 	return builder.String()
// }

// func addMarkdownHeadingFormat(qq gh.Qq) string {
// 	var builder strings.Builder
// 	for _, q := range qq {
// 		if q.Qs != nil {
// 			if q.URL != "" {
// 				builder.WriteString(fmt.Sprintf("#### [%s](%s)\n\n", q.Topic, q.URL))
// 			} else {
// 				builder.WriteString(fmt.Sprintf("#### %s\n\n", q.Topic))
// 			}
//
// 			builder.WriteString(fmt.Sprintf("%s\n", addMarkdownQsFormat(q.Qs)))
// 			builder.WriteString("\n")
// 		}
// 	}
// 	return builder.String()
// }

// 创建基本的 Item
func createBaseItem(repo gh.Repository) *aw.Item {
	name := repo.FullName()
	des := renderReposDes(repo)
	iconPath := renderIcon(repo)

	return wf.NewItem(name).
		Title(name).
		Arg(repo.URL).
		Subtitle(des.String()).
		Copytext(repo.URL).
		Valid(true).
		Autocomplete(name).
		Icon(&aw.Icon{Value: iconPath})
}

// 构建文档 URL
// func buildDocsURL(repo gh.Repository) string {
// 	var docsURL strings.Builder
// 	docsPath := ""
// 	if wf.Config.GetString("docs") == "" {
// 		docsPath = "https://docs.hxha.xyz/x"
// 	}
//
// 	// 构建基础URL部分
// 	if wf != nil {
// 		docsURL.WriteString(fmt.Sprintf("%s/%s#", docsPath, strings.ToLower(repo.Tag)))
// 	} else {
// 		slog.Error("wf is nil", slog.String("repo.Tag", repo.Tag))
// 		docsURL.WriteString(fmt.Sprintf("%s#", strings.ToLower(repo.Tag)))
// 	}
//
// 	// 根据是否有qs判断是否要跳转到其
// 	if repo.Qs == nil {
// 		docsURL.WriteString(strings.ToLower(repo.Type))
// 	} else {
// 		docsURL.WriteString(strings.ToLower(utils.JoinSlashParts(repo.FullName())))
// 	}
//
// 	// 如果是某个repo的rep或者sub，就直接跳转到该sub/rep repo的主repo
//
//
//
// 	return docsURL.String()
// }

func buildDocsURL(repo gh.Repository) string {
	var urlPath strings.Builder

	// 获取基础tag部分
	tag := strings.ToLower(repo.Tag)
	urlPath.WriteString(fmt.Sprintf("%s/#", tag))

	// 如果是子仓库，使用主仓库的路径
	if repo.IsSubRepo() {
		urlPath.WriteString(strings.ToLower(repo.GetMainRepo()))
		return urlPath.String()
	}

	// 如果有问答内容，使用完整仓库名
	if repo.Qs != nil {
		urlPath.WriteString(strings.ToLower(repo.FullName()))
		return urlPath.String()
	}

	// 默认使用类型
	urlPath.WriteString(strings.ToLower(repo.Type))
	return urlPath.String()
}

// 添加修饰键操作
func addModifierActions(item *aw.Item, repo gh.Repository, docsURL string) {
	item.Cmd().Subtitle(fmt.Sprintf("打开该Repo在Docs的URL: %s", docsURL)).Arg(docsURL)
	item.Opt().Subtitle(fmt.Sprintf("复制URL: %s", repo.URL)).Arg(repo.URL)
	item.Shift().Subtitle(fmt.Sprintf("打开文档: %s", repo.Doc)).Arg(repo.Doc)
}

// 主渲染函数
func RenderRepos(repos gh.Repos) (item *aw.Item) {
	for _, repo := range repos {
		// 创建基本项
		item = createBaseItem(repo)

		// 构建文档URL
		docsURL := fmt.Sprintf("%s/%s", wf.Config.GetString("docs"), buildDocsURL(repo))

		// 添加修饰键操作
		addModifierActions(item, repo, docsURL)
	}
	return item
}

// 渲染des
// 也就是item中的subtitle
func renderReposDes(repo gh.Repository) (des strings.Builder) {
	if repo.Type != "" {
		des.WriteString(fmt.Sprintf("【#%s】", repo.Type))
	} else {
		des.WriteString(repo.Des)
	}

	if repo.Des != "" {
		des.WriteString(fmt.Sprintf(" %s", repo.Des))
	}

	return
}

// 渲染remark
// 也就是
// func renderReposRemark(repo gh.Repository) (remark strings.Builder) {
// 	if repo.Des != "" {
// 		remark.WriteString(fmt.Sprintf(" %s", repo.Des))
// 	}
//
// 	// if repo.Pix != nil {
// 	// 	qx := addMarkdownPicFormat(repo.Pix)
// 	// 	remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
// 	// }
//
// 	if repo.Qs != nil {
// 		qx := addMarkdownQsFormat(repo.Qs)
// 		remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
// 	}
//
// 	if repo.Qq != nil {
// 		qx := addMarkdownHeadingFormat(repo.Qq)
// 		remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
// 	}
//
// 	if repo.Cmd != nil {
// 		// var cmds []string
// 		// for _, cmd := range repo.Cmd {
// 		// 	// if len(cmd) > 1 {
// 		// 	// 	cmds = append(cmds, fmt.Sprintf("`%s` %s", cmd[0], cmd[1]))
// 		// 	// } else {
// 		// 	// 	cmds = append(cmds, fmt.Sprintf("`%s`", cmd[0]))
// 		// 	// }
// 		//
// 		// }
// 		qx := addMarkdownCmd(repo.Cmd)
// 		remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
// 	}
// 	return
// }

func renderIcon(repo gh.Repository) (iconPath string) {
	// 因为需要处理有Doc的未加入gh.yml的repo，所以不再判断IsStar
	switch {
	case repo.Qs != nil && repo.Doc != "":
		iconPath = FaQsAndDoc
	case repo.Qs != nil:
		iconPath = FaQs
	case repo.Doc != "":
		iconPath = FaDoc
	case repo.IsStar:
		iconPath = FaStar
	default:
		iconPath = FaRepo
	}
	return
}
