package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/91go/docs-alfred/pkg/qs"

	"github.com/91go/docs-alfred/pkg/gh"
	"github.com/91go/docs-alfred/pkg/goods"
	"github.com/91go/docs-alfred/pkg/ws"
	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

// fCmd represents the f command
var fCmd = &cobra.Command{
	Use:   "f",
	Short: "A brief description of your command",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
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
var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Searching from starred repositories and my repositories",
	Run: func(cmd *cobra.Command, args []string) {
		repos := gh.NewRepos()
		err := repos.ListRepositories(wf.CacheDir() + "/repo.db")
		if err != nil {
			wf.FatalError(err)
		}

		// var ghs []gh.Repository
		// if wf.Cache.Exists(cfgFile) {
		// 	f, err := wf.Cache.Load(cfgFile)
		// 	if err != nil {
		// 		return
		// 	}
		// 	ghs = gh.NewConfigRepos(f).ToRepos()
		// }

		ghs := gh.NewConfigRepos(ReadConfig()).ToRepos()

		repos = append(ghs, repos...)

		for _, repo := range repos {
			url := repo.URL
			var des string
			if repo.Tag != "" {
				des = fmt.Sprintf("[#%s]  %s", repo.Tag, repo.Des)
			} else {
				des = repo.Des
			}
			remark := des
			name := repo.FullName()
			var iconPath string

			if repo.Qs != nil {
				qx := addMarkdownListFormat(repo.Qs)
				remark += fmt.Sprintf("\n \n --- \n \n%s", qx)
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
				remark += fmt.Sprintf("\n \n --- \n \n%s", qx)
				iconPath = FaStar
			}

			if repo.Qs == nil && repo.Cmd == nil {
				if repo.IsStar {
					iconPath = FaCheck
				} else {
					iconPath = FaRepo
				}
			}

			item := wf.NewItem(name).Title(name).
				Arg(remark).
				Subtitle(des).
				Copytext(url).
				Valid(true).
				Autocomplete(name).Icon(&aw.Icon{Value: iconPath})

			// item.Cmd().Subtitle("Preview Description in Markdown Format").Arg(url)
			item.Cmd().Subtitle("Open URL").Arg(url)
		}

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
		// if !wf.Cache.Exists(cfgFile) {
		// 	ErrorHandle(errors.New(cfgFile + " not found"))
		// }
		//
		// f, err := wf.Cache.Load(cfgFile)
		// if err != nil {
		// 	return
		// }
		for _, s := range goods.NewConfigGoods(ReadConfig()) {
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
		// var docs qs.Docs
		// if wf.Cache.Exists(cfgFile) {
		// 	f, err := wf.Cache.Load(cfgFile)
		// 	if err != nil {
		// 		return
		// 	}
		// 	docs = qs.NewConfigQs(f)
		// }
		docs := qs.NewConfigQs(ReadConfig())

		for _, doc := range docs {
			v := doc.Type
			wf.NewItem(v).Title(v).Valid(true).Arg(addMarkdownListFormat(docs.GetQsByName(v))).Autocomplete(v).Subtitle(fmt.Sprintf("[#%s]", doc.Tag))
		}

		if len(args) > 0 {
			wf.Filter(args[0])
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

// wsCmd represents the ws command
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		tks := ws.NewConfigWs(ReadConfig()).SearchWs(args)

		for _, ws := range tks {
			item := wf.NewItem(ws.Name).Title(ws.Name).Subtitle(ws.Des).Valid(true).Quicklook(ws.URL).Autocomplete(ws.Name).Arg(ws.Des).Icon(&aw.Icon{Value: "icons/check.svg"})

			item.Cmd().Subtitle("Open URL").Arg(ws.URL)
		}

		wf.SendFeedback()
	},
}

// ReadConfig Handle Config Not Exists
// panic not effect PreRun()
func ReadConfig() []byte {
	if !wf.Cache.Exists(cfgFile) {
		ErrorHandle(errors.New(cfgFile + " not found"))
	}
	dt, err := wf.Cache.Load(cfgFile)
	if err != nil {
		return nil
	}
	return dt
}
