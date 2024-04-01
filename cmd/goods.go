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
		if wf.Cache.Exists(cfgFile) {
			f, err := wf.Cache.Load(cfgFile)
			if err != nil {
				return
			}
			gd = goods.NewConfigGoods(f)
		}
		for _, s := range gd {

			des := s.Des
			remark := s.Des
			if s.Goods != nil {

				// var data [][]string
				// for _, g := range s.Goods {
				// 	data = append(data, []string{g.Name, g.Param, g.Price, g.Des})
				// }

				// tableString := &strings.Builder{}
				// table := tablewriter.NewWriter(tableString)
				// table.SetHeader([]string{"Name", "Param", "Price", "Des"})
				// table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
				// table.SetCenterSeparator("|")
				// table.AppendBulk(data) // Add Bulk Data
				// table.Render()
				//
				// remark += fmt.Sprintf("\n\n --- \n \n%s", tableString)

				var data []string
				for _, g := range s.Goods {
					data = append(data, fmt.Sprintf("%s[%s]%s: %s", g.Name, g.Param, g.Price, g.Des))
				}
				remark += fmt.Sprintf("\n\n --- \n \n%s", addMarkdownListFormat(data))
			}
			if s.Qs != nil {
				qx := addMarkdownListFormat(s.Qs)
				remark += fmt.Sprintf("\n\n --- \n \n%s", qx)
			}
			wf.NewItem(s.Type).Title(s.Type).Subtitle(fmt.Sprintf("[#%s] %s", s.Tag, des)).Valid(true).Arg(remark)
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
