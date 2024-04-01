package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"github.com/91go/docs-alfred/pkg/gh"
	"gopkg.in/yaml.v3"

	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

const CustomRepo = "gh.yml"

const (
	GhURL      = "https://github.com/"
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
			cmd := exec.Command("./exe", syncJob, "--config=gh.yml")
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		repos, err := gh.NewRepos().ListRepositories(wf.CacheDir() + "/repo.db")
		if err != nil {
			wf.FatalError(err)
		}

		var ghs []gh.Repository
		if wf.Cache.Exists(CustomRepo) {

			f, err := wf.Cache.Load(CustomRepo)
			if err != nil {
				return
			}

			d := yaml.NewDecoder(bytes.NewReader(f))
			for {
				// create new spec here
				spec := new([]gh.Repository)
				// pass a reference to spec reference
				if err := d.Decode(&spec); err != nil {
					// break the loop in case of EOF
					if errors.Is(err, io.EOF) {
						break
					}
					panic(err)
				}
				ghs = append(ghs, *spec...)
			}

			for i, gh := range ghs {
				if strings.Contains(gh.URL, GhURL) {
					sx, _ := strings.CutPrefix(gh.URL, GhURL)
					ghs[i].User = strings.Split(sx, "/")[0]
					ghs[i].Name = strings.Split(sx, "/")[1]
					ghs[i].IsStar = true
				} else {
					log.Printf("Invalid URL: %s", gh.URL)
				}
			}
		}

		repos = append(ghs, repos...)

		for _, repo := range repos.RemoveDuplicates() {
			url := repo.URL
			des := repo.Description
			remark := repo.Description
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
