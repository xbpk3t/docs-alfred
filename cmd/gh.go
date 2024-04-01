package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/91go/docs-alfred/pkg/gh"
	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

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
	PostRun: func(cmd *cobra.Command, args []string) {
		if !wf.IsRunning(syncJob) {
			cmd := exec.Command("./exe", syncJob, fmt.Sprintf("--config=%s", cfgFile))
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		repos := gh.NewRepos()
		err := repos.ListRepositories(wf.CacheDir() + "/repo.db")
		if err != nil {
			wf.FatalError(err)
		}

		var ghs []gh.Repository
		if wf.Cache.Exists(cfgFile) {

			f, err := wf.Cache.Load(cfgFile)
			if err != nil {
				return
			}
			ghs = gh.NewConfigRepos(f).ToRepos()
		}

		repos = append(ghs, repos...)

		for _, repo := range repos.RemoveDuplicates() {
			url := repo.URL
			des := repo.Des
			remark := repo.Des
			name := repo.FullName()
			var iconPath string

			switch {
			case repo.Qs != nil:
				qx := addMarkdownListFormat(repo.Qs)
				remark += fmt.Sprintf("--- \n \n%s", qx)
				iconPath = FaStar
			case repo.Cmd != nil:
				var cmds []string
				for _, cmd := range repo.Cmd {
					if len(cmd) > 1 {
						cmds = append(cmds, fmt.Sprintf("`%s` %s", cmd[0], cmd[1]))
					} else {
						cmds = append(cmds, fmt.Sprintf("`%s`", cmd[0]))
					}
				}
				qx := addMarkdownListFormat(cmds)
				remark += fmt.Sprintf("--- \n \n%s", qx)
				iconPath = FaStar
			case repo.Qs == nil || repo.Cmd == nil:
				if repo.IsStar {
					iconPath = FaCheck
				} else {
					iconPath = FaRepo
				}
			}

			item := wf.NewItem(name).Title(name).
				Arg(url).
				Subtitle(des).
				Copytext(url).
				Valid(true).
				Autocomplete(name).Icon(&aw.Icon{Value: iconPath})

			item.Cmd().Subtitle("Preview Description in Markdown Format").Arg(remark)
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

func init() {
	rootCmd.AddCommand(ghCmd)
}
