package cmd

import (
	"log/slog"
	"os"
	"sync"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/feeds-merge/pkg"
)

var wg sync.WaitGroup

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "feeds-merge",
	Short: "A brief description of your application",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		pkg.NewConfig()

		for _, cate := range pkg.Conf.Categories {
			wg.Add(1)
			go func(cate pkg.Categories) {
				defer wg.Done()
				feedsTitle := cate.Type
				feeds := cate.URLs

				// TODO check feed is valid

				// 拼接urls
				urls := lo.Map(feeds, func(item pkg.Feed, index int) string {
					return item.Feed
				})

				// 移除一些feed为空字符串的item
				urls = lo.Compact(urls)

				allFeeds := pkg.Conf.FetchURLs(urls)
				if combinedFeed, err := pkg.Conf.MergeAllFeeds(feedsTitle, allFeeds); err != nil {
					slog.Info("No feed Found:", slog.String("Feed Type", feedsTitle), slog.Any("Error", err))
				} else {
					atom, err := combinedFeed.ToAtom()
					if err != nil {
						slog.Info("Render RSS Error:", slog.Any("Error", err))
						return
					}
					err = os.WriteFile(feedsTitle+".atom", []byte(atom), os.ModePerm)
					if err != nil {
						slog.Info("Write file Error:", slog.Any("Error", err))
						return
					}
				}
			}(cate)
		}
		wg.Wait()
		os.Exit(0)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.feeds-merge.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
