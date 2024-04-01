package cmd

import (
	"fmt"
	"os/exec"

	"github.com/91go/docs-alfred/pkg/goods"
	"github.com/spf13/cobra"
)

// goodsCmd represents the goods command
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "A brief description of your command",
	PostRun: func(cmd *cobra.Command, args []string) {
		if !wf.IsRunning(syncJob) {
			cmd := exec.Command("./exe", syncJob, fmt.Sprintf("--config=%s", cfgFile))
			if err := wf.RunInBackground(syncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var gd goods.ConfigGoods
		if wf.Cache.Exists(ConfigGoods) {

			f, err := wf.Cache.Load(ConfigGoods)
			if err != nil {
				return
			}
			gd = goods.NewConfigGoods(f)
		}

		for _, s := range gd {
			wf.NewItem(s.Type).Title(s.Type).Subtitle("#" + s.Tag).Valid(false).Arg(s.Des)
		}

		if len(args) > 0 {
			wf.Filter(args[0])
		}

		wf.SendFeedback()
	},
}

func init() {
	rootCmd.AddCommand(goodsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// goodsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// goodsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
