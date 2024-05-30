package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/goods"
	"github.com/spf13/cobra"
)

// goodsCmd represents the goods command
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		configGoods := goods.NewConfigGoods(f)

		var res strings.Builder

		// goodsMap := make(map[string]goods.ConfigGoods)

		// for _, g := range configGoods {
		//
		// 	if _, exists := goodsMap[g.Tag]; !exists {
		// 		goodsMap[g.Tag] = goods.ConfigGoods{g}
		// 	} else {
		// 		goodsMap[g.Tag] = append(goodsMap[g.Tag], g)
		// 	}
		// }
		//
		// for tag, goodsInfo := range goodsMap {
		// 	res.WriteString(fmt.Sprintf("## %s \n", tag))
		//
		// 	for _, gi := range goodsInfo {
		// 		res.WriteString(fmt.Sprintf("\n ### %s \n", gi.Type))
		//
		// 		// data := make([][]string, len(gi.Goods))
		// 		// for _, g := range gi.Goods {
		// 		// 	data = append(data, []string{g.Name, g.Param, g.Price, g.Des})
		// 		// }
		//
		// 		// tableString := &strings.Builder{}
		// 		// table := tablewriter.NewWriter(tableString)
		// 		// table.SetHeader([]string{"Name", "Param", "Price", "Des"})
		// 		// table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		// 		// table.SetCenterSeparator("|")
		// 		// table.AppendBulk(data)
		// 		// table.Render()
		// 		//
		// 		// res.WriteString(fmt.Sprintf("\n\n%s \n", tableString))
		//
		// 		for _, g := range gi.Goods {
		// 			name := ""
		// 			if g.Use {
		// 				name = fmt.Sprintf("- ***%s*** \n", g.Name)
		// 			} else {
		// 				name = fmt.Sprintf("- %s \n", g.Name)
		// 			}
		// 			res.WriteString(name)
		// 		}
		//
		// 		// qs
		// 		for _, q := range gi.Qs {
		// 			res.WriteString(fmt.Sprintf("- %s \n", q))
		// 		}
		// 	}
		// }

		var ss []string

		for _, gi := range configGoods {
			if !slices.Contains(ss, gi.Tag) {
				ss = append(ss, gi.Tag)
				res.WriteString(fmt.Sprintf("## %s \n", gi.Tag))
				res.WriteString(fmt.Sprintf("### %s \n", gi.Type))
				mark := ""
				if gi.Goods[0].Use {
					mark = "***"
				} else {
					mark = "~~"
				}
				res.WriteString(fmt.Sprintf("- %s[%s]%s\n", mark, gi.Goods[0].Name, mark))
			} else {
				res.WriteString(fmt.Sprintf("### %s \n", gi.Type))
				for _, g := range gi.Goods {
					mark := ""
					if g.Use {
						mark = "***"
					} else {
						mark = "~~"
					}
					res.WriteString(fmt.Sprintf("- %s[%s]%s\n", mark, g.Name, mark))
				}
				// qs
				for _, q := range gi.Qs {
					res.WriteString(fmt.Sprintf("- %s \n", q))
				}
			}
		}

		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to goods.md")
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
