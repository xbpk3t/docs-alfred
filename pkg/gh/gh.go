package gh

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	GhURL = "https://github.com/"
)

type ConfigRepos []struct {
	Type  string       `yaml:"type"`
	Repos []Repository `yaml:"repo"`
	Qs    []string     `yaml:"qs"`
}

type Repository struct {
	LastUpdated time.Time
	Doc         string `yaml:"doc,omitempty"`
	URL         string `yaml:"url"`
	Name        string `yaml:"name,omitempty"`
	User        string
	Des         string     `yaml:"des,omitempty"`
	Tag         string     // used to mark Type
	Qs          []string   `yaml:"qs,omitempty"`
	Cmd         [][]string `yaml:"cmd,omitempty"`
	Use         []string   `yaml:"use,omitempty"`
	IsStar      bool
}

type Repos []Repository

func NewConfigRepos(f []byte) ConfigRepos {
	var ghs ConfigRepos

	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigRepos)
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

	return ghs
}

// ToRepos Convert Type to Repo
func (cr ConfigRepos) ToRepos() Repos {
	repos := make(Repos, 0)
	for _, config := range cr {
		for _, repo := range config.Repos {
			if strings.Contains(repo.URL, GhURL) {
				sx, _ := strings.CutPrefix(repo.URL, GhURL)
				repo.User = strings.Split(sx, "/")[0]
				repo.Name = strings.Split(sx, "/")[1]
				repo.IsStar = true
				repo.Tag = config.Type
				repos = append(repos, repo)
			} else {
				log.Printf("Invalid URL: %s", repo.URL)
			}
		}
	}

	return repos
}