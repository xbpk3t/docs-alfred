package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/utils"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/goods"
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
				res.WriteString(addMarkdownFormat(gi, &res))

			} else {
				res.WriteString(fmt.Sprintf("### %s \n", gi.Type))
				res.WriteString(addMarkdownFormat(gi, &res))

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

		targetFile := utils.ChangeFileExtFromYamlToMd(cfgFile)
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

// addMarkdownFormat 将ConfigGoodsX中的GoodItem转换为Markdown格式的字符串
func addMarkdownFormat(goodsX goods.ConfigGoodsX, res *strings.Builder) string {

	for _, gd := range goodsX.Goods {

		summary := formatSummaryForGoods(gd)
		// details := gd.Des
		details := formatDetailsForGoods(gd)

		if details == "" {
			res.WriteString(fmt.Sprintf("- %s\n", summary))
		} else {
			res.WriteString(fmt.Sprintf("\n\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n", summary, details))
		}
	}

	return res.String()
}

// formatSummaryForGoods 格式化摘要
func formatSummaryForGoods(gd goods.GoodsX) string {
	var mark string

	if gd.Use {
		mark = "***"
	} else {
		mark = "~~"
	}

	if gd.URL != "" {
		return fmt.Sprintf("%s[%s](%s)%s", mark, gd.Name, gd.URL, mark)
	}
	return fmt.Sprintf("%s%s%s", mark, gd.Name, mark)
}

func formatDetailsForGoods(gd goods.GoodsX) (res string) {
	if gd.Param != "" {
		res += fmt.Sprintf("【Param】%s", gd.Param)
	}
	if gd.Price != "" {
		res += fmt.Sprintf("【Price】%s", gd.Price)
	}
	if gd.Date != nil {
		res += fmt.Sprintf("【购买时间】%s", strings.Join(gd.Date, ","))
	}
	if gd.Des != "" {
		res += "---"
		res += gd.Des
	}

	return res
}
