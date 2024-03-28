package cmd

import (
	"errors"
	"os/exec"

	"github.com/91go/docs-alfred/pkg"
	aw "github.com/deanishe/awgo"

	"github.com/spf13/cobra"
)

// wsCmd represents the ws command
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	PostRun: func(cmd *cobra.Command, args []string) {
		if !wf.IsRunning(syncJob) {
			cmd := exec.Command("./exe", syncJob, "--config=ws.yml")
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		wsFile := wf.Cache.Exists(cfgFile)
		if !wsFile {
			ErrorHandle(errors.New(cfgFile + " not exist"))
		}

		tks := pkg.SearchWebstack(wf.CacheDir()+"/ws.yml", args)
		for _, ws := range tks {
			wf.NewItem(ws.Name).Title(ws.Name).Subtitle(ws.Des).Valid(true).Quicklook(ws.URL).Autocomplete(ws.Name).Arg(ws.URL).Icon(&aw.Icon{Value: "icons/check.svg"}).Copytext(ws.URL).Cmd().Subtitle("Press Enter to copy this url to clipboard")
		}

		// if len(args) > 0 {
		// 	wf.Filter(args[0])
		// }

		wf.SendFeedback()
	},
}

func init() {
	rootCmd.AddCommand(wsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// wsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// wsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
