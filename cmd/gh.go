package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

const CustomRepo = "gh.yml"

const (
	GhURL      = "https://github.com/"
	GistSearch = "https://gist.github.com/search?q=%s"
	RepoSearch = "https://github.com/search?q=%s&type=repositories"
)

type Repository struct {
	LastUpdated time.Time
	URL         string `yaml:"url"`
	Name        string
	User        string
	Description string     `yaml:"des,omitempty"`
	Qs          []string   `yaml:"qs"`
	Cmd         [][]string `yaml:"cmd,omitempty"`
	IsStar      bool
}

func (r Repository) FullName() string {
	return fmt.Sprintf("%s/%s", r.User, r.Name)
}

// ghCmd represents the repo command
var ghCmd = &cobra.Command{
	Use:     "gh",
	Short:   "Searching from starred repositories and my repositories",
	Example: "icons/repo.png",
	PostRun: func(cmd *cobra.Command, args []string) {
		if !wf.IsRunning(syncJob) {
			cmd := exec.Command("./exe", syncJob, "--config=gh.yml")
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// repos, err := ListRepositories()
		// if err != nil {
		// 	wf.FatalError(err)
		// }

		var ghs []Repository
		if wf.Cache.Exists(CustomRepo) {

			f, err := wf.Cache.Load(CustomRepo)
			if err != nil {
				return
			}

			d := yaml.NewDecoder(bytes.NewReader(f))
			for {
				// create new spec here
				spec := new([]Repository)
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

		// repos = append(ghs, repos...)

		for _, repo := range removeDuplicates(ghs) {
			url := repo.URL
			des := repo.Description
			name := repo.FullName()

			if repo.Qs != nil {
				qx := addMarkdownListFormat(repo.Qs)
				des += fmt.Sprintf("--- \n \n%s", qx)
			}
			if repo.Cmd != nil {
				var cmds []string
				for _, cmd := range repo.Cmd {
					if len(cmd) > 1 {
						cmds = append(cmds, fmt.Sprintf("`%s` %s", cmd[0], cmd[1]))
					} else {
						cmds = append(cmds, cmd[0])
					}
				}
				qx := addMarkdownListFormat(cmds)
				des += fmt.Sprintf("--- \n \n%s", qx)
			}

			item := wf.NewItem(name).Title(name).
				Arg(url).
				Subtitle(des).
				Copytext(url).
				Valid(true).
				Autocomplete(name)

			if repo.Qs != nil || repo.Cmd != nil {
				item.Icon(&aw.Icon{Value: "icons/check.svg"})
			} else {
				item.Icon(&aw.Icon{Value: "icons/repo.png"})
			}

			item.Cmd().Subtitle("Preview Description in Markdown Format").Arg(des)
		}

		if len(args) > 0 {
			wf.Filter(args[0])
			// wf.Feedback.Filter(args[0], fuzzy.Sorter{})
		}

		wf.NewItem("Search Github").
			Arg(fmt.Sprintf(RepoSearch, strings.Join(args, "+"))).
			Valid(true).
			Icon(&aw.Icon{Value: "icons/search.svg"}).
			Title(fmt.Sprintf("Search Github For '%s'", strings.Join(args, " ")))
		wf.NewItem("Search Gist").
			Arg(fmt.Sprintf(GistSearch, strings.Join(args, "+"))).
			Valid(true).
			Icon(&aw.Icon{Value: "icons/gists.png"}).
			Title(fmt.Sprintf("Search Gist For '%s'", strings.Join(args, " ")))
		wf.SendFeedback()
	},
}

func init() {
	rootCmd.AddCommand(ghCmd)
}

// Search from sqlite
// func ListRepositories() ([]Repository, error) {
// 	db, err := utils.OpenDB(wf.CacheDir() + "/repo.db")
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	rows, err := db.Query("SELECT id, url,description, name,user,updated_at FROM repository")
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var repos []Repository
//
// 	for rows.Next() {
// 		var id, url, descr, name, user string
// 		var updated time.Time
// 		err = rows.Scan(&id, &url, &descr, &name, &user, &updated)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		repos = append(repos, Repository{
// 			URL:         url,
// 			Name:        name,
// 			User:        user,
// 			Description: descr,
// 			LastUpdated: updated,
// 			IsStar:      false,
// 		})
// 	}
//
// 	return repos, nil
// }

func removeDuplicates(ts []Repository) []Repository {
	uniqueValues := make(map[string]bool)
	result := make([]Repository, 0)

	for _, t := range ts {
		if !uniqueValues[t.URL] {
			uniqueValues[t.URL] = true
			result = append(result, t)
		}
	}

	return result
}

// func addMarkdownListFormat(str []string) string {
// 	var builder strings.Builder
// 	for _, str := range str {
// 		builder.WriteString(fmt.Sprintf("- %s\n", str))
// 	}
// 	return builder.String()
// }
