package cmd

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/samber/lo"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

const syncJob = "sync"

const QsFile = "qs.yml"

type Doc struct {
	Name string `yaml:"name,omitempty"`
	Cate string `yaml:"cate,omitempty"`
	Xxx  []Xxx  `yaml:"xxx,omitempty"`
}

type Xxx struct {
	Qs string `yaml:"qs,omitempty"`
	As string `yaml:"as,omitempty"`
}

type Docs []Doc

// qsCmd represents the qs command
var qsCmd = &cobra.Command{
	Use:   "qs",
	Short: "A brief description of your command",
	PostRun: func(cmd *cobra.Command, args []string) {
		if !wf.IsRunning(syncJob) {
			cmd := exec.Command("./exe", syncJob)
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		docs := NewDocs()

		switch len(args) {
		case 0:
			// default: display all name
			for _, v := range docs.GetNames() {
				wf.NewItem(v).Title(v).Valid(false).Arg(v).Autocomplete(v)
			}
			wf.SendFeedback()
		case 1:
			// determine
			query := args[0]
			var qss []Xxx
			if docs.IsHitName(query) {
				qss = docs.GetQsByName(query)
			} else {
				qss = docs.SearchQs(query)
			}

			for _, qs := range qss {
				wf.NewItem(qs.Qs).Title(qs.Qs).Valid(true).Arg(qs.As).Autocomplete(qs.As).Subtitle(qs.As)
			}

			wf.Filter(query)
			wf.SendFeedback()
		case 2:
			// vv name <qs>
			name := args[0]
			query := args[1]

			qss := docs.GetQsByName(name)
			for _, qs := range qss {
				wf.NewItem(qs.Qs).Title(qs.Qs).Valid(true).Arg(qs.As).Autocomplete(qs.As).Subtitle(qs.As)
			}
			wf.Filter(query)
			wf.SendFeedback()
		}
	},
}

func init() {
	rootCmd.AddCommand(qsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// qsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// qsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

var once sync.Once

func NewDocs() Docs {
	var docs Docs

	once.Do(func() {
		if wf.Cache.Exists(QsFile) {
			f, err := wf.Cache.Load(QsFile)
			if err != nil {
				return
			}
			d := yaml.NewDecoder(bytes.NewReader(f))
			for {
				spec := new(Doc)
				if err := d.Decode(&spec); err != nil {
					// break the loop in case of EOF
					if errors.Is(err, io.EOF) {
						break
					}
					panic(err)
				}
				if spec != nil {
					docs = append(docs, *spec)
				}
			}
		}
	})
	return docs
}

// GetNames Get All Names
func (d Docs) GetNames() (names []string) {
	for _, doc := range d {
		names = append(names, doc.Name)
	}
	return
}

// func (d Docs) GetNameByCate(cate string) (names []string) {
// 	for _, doc := range d {
// 		if doc.Cate == cate {
// 			names = append(names, doc.Name)
// 		}
// 	}
// 	return
// }
//

func (d Docs) IsHitName(query string) bool {
	return lo.ContainsBy(d, func(item Doc) bool {
		return strings.EqualFold(item.Name, query)
	})
}

func (d Docs) GetQsByName(name string) (qs []Xxx) {
	for _, doc := range d {
		if doc.Name == name {
			qs = append(qs, doc.Xxx...)
		}
	}
	return
}

func (d Docs) SearchQs(query string) (qs []Xxx) {
	for _, doc := range d {
		for _, xxx := range doc.Xxx {
			if strings.Contains(xxx.Qs, query) {
				qs = append(qs, xxx)
			}
		}
	}
	return
}
