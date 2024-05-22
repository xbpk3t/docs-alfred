package cmd

import (
	"errors"
	"fmt"
	"github.com/91go/docs-alfred/pkg/ws"
	"log/slog"
	"net/url"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/91go/docs-alfred/pkg/qs"

	"github.com/91go/docs-alfred/pkg/gh"
	"github.com/91go/docs-alfred/pkg/goods"
	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

// fCmd represents the f command
var fCmd = &cobra.Command{
	Use:   "f",
	Short: "A brief description of your command",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// ReadConfig Handle Config Not Exists
		// panic not effect PreRun()
		if !wf.Cache.Exists(cfgFile) {
			ErrorHandle(errors.New(cfgFile + " not found"))
		}
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
	fCmd.AddCommand(goodsCmd)
	fCmd.AddCommand(qsCmd)
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
	GistSearch = "https://gist.github.com/search?q=%s"
	RepoSearch = "https://github.com/search?q=%s&type=repositories"
	FaCheck    = "icons/check.svg"
	FaGists    = "icons/gists.png"
	FaRepo     = "icons/repo.png"
	FaSearch   = "icons/search.svg"
	FaStar     = "icons/star.svg"
)

// ghCmd represents the repo command
// Enter          直接打开URL
// CMD + Enter    Markdown View
// Ctrl + Enter   Copy URL
// Shift + Enter  打开在docs项目对应heading的URL
var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Searching from starred repositories and my repositories",
	Run: func(cmd *cobra.Command, args []string) {
		repos := gh.NewRepos()
		err := repos.ListRepositories(wf.CacheDir() + "/repo.db")
		if err != nil {
			wf.FatalError(err)
		}

		ghs := gh.NewConfigRepos(data).ToRepos()

		repos = append(ghs, repos...)

		if len(args) > 0 && strings.HasPrefix(args[0], "#") {
			tags := repos.ExtractTags()

			// if hit tag
			ptag := strings.TrimPrefix(args[0], "#")
			if slices.Contains(tags, ptag) {
				// for _, tagRepo := range  {
				// 	name := tagRepo.FullName()
				// 	wf.NewItem(name).Title(name).Valid(false).Autocomplete(name)
				// }
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
		wf.NewItem("Search Gist").
			Arg(fmt.Sprintf(GistSearch, strings.Join(args, "+"))).
			Valid(true).
			Icon(&aw.Icon{Value: FaGists}).
			Title(fmt.Sprintf("Search Gist For '%s'", strings.Join(args, " ")))
		wf.SendFeedback()
	},
}

// goodsCmd represents the goods command
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {

		//
		goods := goods.NewConfigGoods(data)

		for _, s := range goods {
			des := s.Des
			remark := s.Des
			if s.Goods != nil {

				// var data [][]string
				// for _, g := range s.Goods {
				// 	data = append(data, []string{g.Type, g.Param, g.Price, g.Des})
				// }

				// tableString := &strings.Builder{}
				// table := tablewriter.NewWriter(tableString)
				// table.SetHeader([]string{"Type", "Param", "Price", "Des"})
				// table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
				// table.SetCenterSeparator("|")
				// table.AppendBulk(data) // Add Bulk Data
				// table.Render()
				//
				// remark += fmt.Sprintf("\n\n --- \n \n%s", tableString)

				var data []string
				for _, g := range s.Goods {
					data = append(data, fmt.Sprintf("%s[%s]%s: %s", g.Name, g.Param, g.Price, g.Des))
				}
				remark += fmt.Sprintf("\n\n --- \n \n%s", addMarkdownListFormat(data))
			}
			if s.Qs != nil {
				qx := addMarkdownListFormat(s.Qs)
				remark += fmt.Sprintf("\n\n --- \n \n%s", qx)
			}
			wf.NewItem(s.Type).Title(s.Type).Subtitle(fmt.Sprintf("[#%s] %s", s.Tag, des)).Valid(true).Arg(remark)
		}

		if len(args) > 0 {
			wf.Filter(args[0])
		}

		wf.SendFeedback()
	},
}

// qsCmd represents the qs command
var qsCmd = &cobra.Command{
	Use:   "qs",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		docs := qs.NewConfigQs(data)

		for _, doc := range docs {
			v := doc.Type
			mdList := addMarkdownListFormat(docs.GetQsByName(v))
			wf.NewItem(v).Title(v).Valid(true).Arg(mdList).Autocomplete(v).Subtitle(fmt.Sprintf("[#%s]", doc.Tag))
		}

		if len(args) > 0 {
			wf.Filter(args[0])
		}
		wf.SendFeedback()
	},
}

// wsCmd represents the ws command
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		tks := ws.NewConfigWs(data).SearchWs(args)

		for _, ws := range tks {
			item := wf.NewItem(ws.Name).Title(ws.Name).Subtitle(ws.Des).Valid(true).Quicklook(ws.URL).Autocomplete(ws.Name).Arg(ws.Des).Icon(&aw.Icon{Value: "icons/check.svg"})

			item.Cmd().Subtitle(fmt.Sprintf("Open URL: %s", ws.URL)).Arg(ws.URL)
			item.Opt().Subtitle(fmt.Sprintf("Copy URL: %s", ws.URL)).Arg(ws.URL)
		}

		wf.SendFeedback()
	},
}

func addMarkdownListFormat(str []string) string {
	var builder strings.Builder
	for _, str := range str {
		builder.WriteString(fmt.Sprintf("- %s\n", str))
	}
	return builder.String()
}

func addMarkdownQsFormat(qs gh.Qs) string {
	var builder strings.Builder
	// builder.WriteString("<dl>")
	// for _, q := range qs {
	//
	// 	builder.WriteString(fmt.Sprintf("- %s \n", q.Q))
	// 	builder.WriteString(fmt.Sprintf("\n %s \n", q.X))
	// 	// builder.WriteString(fmt.Sprintf("<dt>%s</dt>", q.Q))
	// 	// builder.WriteString(fmt.Sprintf("<dd>%s</dd>", q.X))
	// }
	// // builder.WriteString("</dl>")
	//
	// return builder.String()

	for _, q := range qs {
		builder.WriteString(fmt.Sprintf("- %s\n", q.Q))
	}
	return builder.String()
}

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

func addMarkdownPicFormat(URLs []string) string {
	var builder strings.Builder
	for _, u := range URLs {
		name, _ := GetFileNameFromURL(u)
		builder.WriteString(fmt.Sprintf("- [%s](%s)\n", name, u))
	}
	return builder.String()
}

func addMarkdownHeadingFormat(qq gh.Qq) string {
	var builder strings.Builder
	for _, q := range qq {
		if q.Qs != nil {
			if q.URL != "" {
				builder.WriteString(fmt.Sprintf("#### [%s](%s)\n\n", q.Topic, q.URL))
			} else {
				builder.WriteString(fmt.Sprintf("#### %s\n\n", q.Topic))
			}

			builder.WriteString(fmt.Sprintf("%s\n", addMarkdownQsFormat(q.Qs)))
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func RenderRepos(repos gh.Repos) (item *aw.Item) {
	for _, repo := range repos {
		repoURL := repo.URL
		var des string
		if repo.Tag != "" {
			des = fmt.Sprintf("[#%s]  %s", repo.Tag, repo.Des)
		} else {
			des = repo.Des
		}
		var remark strings.Builder
		remark.WriteString(des)
		name := repo.FullName()
		var iconPath string

		if repo.Pix != nil {
			qx := addMarkdownPicFormat(repo.Pix)
			remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
			iconPath = FaStar
		}

		if repo.Qs != nil {
			qx := addMarkdownQsFormat(repo.Qs)
			remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
			iconPath = FaStar
		}

		if repo.Qq != nil {
			qx := addMarkdownHeadingFormat(repo.Qq)
			remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
			iconPath = FaStar
		}

		if repo.Cmd != nil {
			var cmds []string
			for _, cmd := range repo.Cmd {
				if len(cmd) > 1 {
					cmds = append(cmds, fmt.Sprintf("`%s` %s", cmd[0], cmd[1]))
				} else {
					cmds = append(cmds, fmt.Sprintf("`%s`", cmd[0]))
				}
			}
			qx := addMarkdownListFormat(cmds)
			remark.WriteString(fmt.Sprintf("\n \n --- \n \n%s", qx))
			iconPath = FaStar
		}

		if repo.Qs == nil && repo.Cmd == nil {
			if repo.IsStar {
				iconPath = FaCheck
			} else {
				iconPath = FaRepo
			}
		}

		item = wf.NewItem(name).Title(name).
			Arg(remark.String()).
			Subtitle(des).
			Copytext(repoURL).
			Valid(true).
			Autocomplete(name).Icon(&aw.Icon{Value: iconPath})

		item.Cmd().Subtitle(fmt.Sprintf("Open URL: %s", repoURL)).Arg(repoURL)
		item.Opt().Subtitle(fmt.Sprintf("Copy URL: %s", repoURL)).Arg(repoURL)
	}
	return item
}
