package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/goods"
	"github.com/xbpk3t/docs-alfred/utils"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// goodsCmd represents the goods command
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "Generate Markdown documentation from goods configuration",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			slog.Error("Error reading config file", slog.String("error", err.Error()))
			return
		}

		configGoods := goods.NewConfigGoods(data)
		var builder strings.Builder
		var seenTags []string

		for _, gi := range configGoods {
			if !slices.Contains(seenTags, gi.Tag) {
				seenTags = append(seenTags, gi.Tag)
				builder.WriteString(fmt.Sprintf("## %s\n", gi.Tag))
			}
			builder.WriteString(fmt.Sprintf("### %s\n", gi.Type))
			builder.WriteString(goods.AddMarkdownFormat(gi))
			builder.WriteString(goods.AddTypeQs(gi))
		}

		targetFile := utils.ChangeFileExtFromYamlToMd(cfgFile)
		err = os.WriteFile(targetFile, []byte(builder.String()), os.ModePerm)
		if err != nil {
			slog.Error("Error writing Markdown file", slog.String("error", err.Error()))
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

func init() {
	rootCmd.AddCommand(goodsCmd)
}
