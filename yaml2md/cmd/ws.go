package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/ws"

	"github.com/spf13/cobra"
)

// wsCmd represents the ws command
var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		dfo, _ := ws.NewConfigWs(f)
		var res strings.Builder

		for _, urls := range dfo {
			res.WriteString(fmt.Sprintf("\n## %s \n\n", urls.Type))
			for _, url := range urls.URLs {
				if url.Name == "" {
					url.Name = url.URL
				}
				// res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.URL, url.Des))
				if url.URL != "" {
					res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.URL, url.Des))
				} else {
					res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.Feed, url.Des))
				}
			}
		}

		err = os.WriteFile(targetFile, []byte(res.String()), os.ModePerm)
		if err != nil {
			return
		}

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
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
