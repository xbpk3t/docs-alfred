package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/feed"

	"github.com/spf13/cobra"
)

// feedsCmd represents the feeds command
var feedsCmd = &cobra.Command{
	Use:   "feeds",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
		if err != nil {
			return
		}

		dfo, _ := feed.NewConfigFeeds(f)
		var res strings.Builder

		for _, feeds := range dfo {
			res.WriteString(fmt.Sprintf("\n## %s \n\n", feeds.Type))
			for _, feed := range feeds.Feeds {
				if feed.Name == "" {
					feed.Name = feed.URL
				}
				if feed.URL != "" {
					res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", feed.Name, feed.URL, feed.Des))
				} else {
					res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", feed.Name, feed.Feed, feed.Des))
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
	rootCmd.AddCommand(feedsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// feedsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// feedsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
