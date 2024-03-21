package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/91go/yaml2md/qs"
	"github.com/spf13/cobra"
)

const syncJob = "sync"

const QsFolder = "qs"

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
		docs := qs.NewDocs(wf.CacheDir() + "/qs.yml")

		switch len(args) {
		case 0:
			// default: display all name
			for _, doc := range docs {
				for _, xxx := range doc.Xxx {
					v := xxx.Name
					wf.NewItem(v).Title(v).
						Valid(true).
						Arg(addMarkdownListFormat(xxx.Qs)).
						Autocomplete(v).
						Subtitle(fmt.Sprintf("#%s", doc.Cate))

					// wf.NewItem(v).Title(v).Valid(true).Arg("111").Autocomplete(v).Subtitle(fmt.Sprintf("#%s", doc.Cate))
				}
			}

			wf.SendFeedback()

			// case 1:
			// 	// determine
			// 	query := args[0]
			// 	// var qss []string
			// 	// if docs.IsHitName(query) {
			// 	// 	qss = docs.GetQsByName(query)
			// 	// } else {
			// 	// 	qss = docs.SearchQs(query)
			// 	// }
			// 	//
			// 	// for _, qs := range qss {
			// 	// 	wf.NewItem(qs).Title(qs).Valid(true).Arg(qs)
			// 	// }
			//
			// 	for _, qs := range docs.GetQsByName(query) {
			// 		wf.NewItem(qs).Title(qs).Valid(true).Arg(qs)
			// 	}
			//
			// 	// wf.Filter(query)
			// 	wf.SendFeedback()

			// case 2:
			// 	// vv name <qs>
			// 	name := args[0]
			// 	query := args[1]
			//
			// 	qss := docs.GetQsByName(name)
			// 	for _, qs := range qss {
			// 		wf.NewItem(qs).Title(qs).Valid(true).Arg(qs)
			// 	}
			// 	wf.Filter(query)
			// 	wf.SendFeedback()
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

func addMarkdownListFormat(str []string) string {
	var builder strings.Builder
	for _, str := range str {
		builder.WriteString(fmt.Sprintf("- %s\n", str))
	}
	return builder.String()
}
