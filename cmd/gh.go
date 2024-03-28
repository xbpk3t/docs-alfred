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

	gh "github.com/91go/docs-alfred/pkg/gh"
	"github.com/google/go-github/v56/github"

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
		repos, err := ListRepositories()
		if err != nil {
			wf.FatalError(err)
		}

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

		repos = append(ghs, repos...)

		for _, repo := range removeDuplicates(repos) {
			url := repo.URL
			des := repo.Description
			remark := repo.Description
			name := repo.FullName()
			var iconPath string

			switch {
			case repo.Qs != nil:
				qx := addMarkdownListFormat(repo.Qs)
				remark += fmt.Sprintf("--- \n \n%s", qx)
				iconPath = "icons/star.svg"
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
				iconPath = "icons/star.svg"
			case repo.Qs == nil || repo.Cmd == nil:
				if repo.IsStar {
					iconPath = "icons/check.svg"
				} else {
					iconPath = "icons/repo.svg"
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
func ListRepositories() ([]Repository, error) {
	db, err := gh.OpenDB(wf.CacheDir() + "/repo.db")
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT id, url,description, name,user,updated_at FROM repository")
	if err != nil {
		return nil, err
	}

	var repos []Repository

	for rows.Next() {
		var id, url, descr, name, user string
		var updated time.Time
		err = rows.Scan(&id, &url, &descr, &name, &user, &updated)
		if err != nil {
			return nil, err
		}

		repos = append(repos, Repository{
			URL:         url,
			Name:        name,
			User:        user,
			Description: descr,
			LastUpdated: updated,
			IsStar:      false,
		})
	}

	return repos, nil
}

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

func UpdateRepositories(token string) (int64, error) {
	// my repos
	userRepos, err := gh.NewGithubClient(token).ListUserRepositories()
	if err != nil {
		return 0, err
	}

	// starred repos
	starredRepos, err := gh.NewGithubClient(token).ListStarredRepositories()
	if err != nil {
		return 0, err
	}

	db, err := gh.OpenDB(wf.CacheDir() + "/repo.db")
	if err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	found := map[string]struct{}{}
	counter := int64(0)

	for _, repo := range append(userRepos, starredRepos...) {
		log.Printf("Updating %s/%s", *repo.Owner.Login, *repo.Name)

		name := fmt.Sprintf("%s/%s", *repo.Owner.Login, *repo.Name)
		res, err := db.Exec(
			`INSERT OR REPLACE INTO repository (
					id,
					url,
					description,
					name, user,
					pushed_at,
					updated_at,
					created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			name,
			nilableString(repo.HTMLURL),
			nilableString(repo.Description),
			*repo.Name,
			*repo.Owner.Login,
			githubTime(repo.PushedAt),
			githubTime(repo.UpdatedAt),
			githubTime(repo.CreatedAt),
		)
		if err != nil {
			return counter, err
		}
		found[name] = struct{}{}
		rows, _ := res.RowsAffected()
		counter += rows
	}

	existing, err := ListRepositories()
	if err != nil {
		return 0, err
	}

	// purge repos that don't exit any more
	for _, repo := range existing {
		if _, exists := found[repo.FullName()]; !exists {
			log.Printf("Repo %s doesn't exist, deleting", repo.FullName())

			_, err := db.Exec(
				`DELETE FROM repository WHERE id=?`,
				repo.FullName(),
			)
			if err != nil {
				return 0, err
			}

		}
	}

	return counter, tx.Commit()
}

func nilableString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func githubTime(t *github.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	return &t.Time
}
