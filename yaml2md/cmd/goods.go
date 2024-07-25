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
		var ss []string
		for _, gi := range configGoods {
			// 顺序渲染
			if !slices.Contains(ss, gi.Tag) {
				ss = append(ss, gi.Tag)
				res.WriteString(fmt.Sprintf("## %s \n", gi.Tag))
				res.WriteString(fmt.Sprintf("### %s \n", gi.Type))
				mark := ""

				for _, gd := range gi.Goods {

					if gd.Use {
						mark = "***"
					} else {
						mark = "~~"
					}
					if gd.Des != "" {
						if gd.URL == "" {
							res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s%s%s</summary>\n\n%s\n\n</details>\n\n", mark, gd.Name, mark, gd.Des))
						} else {
							res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s[%s](%s)%s</summary>\n\n%s\n\n</details>\n\n", mark, gd.Name, gd.URL, mark, gd.Des))
						}
					} else {
						res.WriteString(fmt.Sprintf("- %s%s%s\n", mark, gd.Name, mark))
					}
				}

			} else {
				res.WriteString(fmt.Sprintf("### %s \n", gi.Type))
				for _, g := range gi.Goods {
					mark := ""
					if g.Use {
						mark = "***"
					} else {
						mark = "~~"
					}
					if g.Des != "" {
						if g.URL == "" {
							res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s%s%s</summary>\n\n%s\n\n</details>\n\n", mark, g.Name, mark, g.Des))
						} else {
							res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s[%s](%s)%s</summary>\n\n%s\n\n</details>\n\n", mark, g.Name, g.URL, mark, g.Des))
						}

					} else {
						res.WriteString(fmt.Sprintf("- %s%s%s\n", mark, g.Name, mark))
					}
				}

				if gi.Qs != nil {
					res.WriteString("--- \n")
				}

				// qs
				for _, q := range gi.Qs {
					if q.X != "" {
						res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", q.Q, q.X))
					} else {
						res.WriteString(fmt.Sprintf("- %s \n", q.Q))
					}
				}
			}
		}

		tf, _ := strings.CutSuffix(cfgFile, ".yml")
		targetFile := fmt.Sprintf("%s.md", tf)
		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
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
