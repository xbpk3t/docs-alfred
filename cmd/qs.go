package cmd

import (
	"bytes"
	"errors"
	"io"
	"os/exec"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const syncJob = "sync"

const QsFile = "qs.yml"

type Docs struct {
	Name string `yaml:"name,omitempty"`
	Cate string `yaml:"cate,omitempty"`
	Xxx  []struct {
		Qs string `yaml:"qs,omitempty"`
		As string `yaml:"as,omitempty"`
	} `yaml:"xxx,omitempty"`
}

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
		var docs []Docs
		if wf.Cache.Exists(QsFile) {
			f, err := wf.Cache.Load(QsFile)
			if err != nil {
				return
			}
			d := yaml.NewDecoder(bytes.NewReader(f))
			for {
				spec := new(Docs)
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

		for _, doc := range docs {
			for _, q := range doc.Xxx {
				wf.NewItem(q.Qs).Title(q.Qs).Subtitle(q.As).Valid(false)
			}
			// wf.NewItem(doc.Name).Title(doc.Name).Valid(false)
		}

		// if len(args) > 0 {
		// 	wf.Filter(args[0])
		// }

		wf.SendFeedback()
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
