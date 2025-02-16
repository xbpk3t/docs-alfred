package cmd

import (
	"errors"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/spf13/cobra"
)

const SyncJob = "sync"

const expire = 60

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		url := wf.Config.GetString("url") + cfgFile
		slog.Info("Fetching URL...", slog.String("URL", url))

		if url == "" {
			slog.Info("URL is Empty", slog.Any("URL", url))
			ErrorHandle(errors.New("URL is Empty"))
		}

		client := resty.New()

		_, err := wf.Cache.LoadOrStore(cfgFile, time.Duration(expire)*time.Minute, func() ([]byte, error) {
			data, err := client.R().Get(url)
			if err != nil {
				return nil, err
			}

			slog.Info("Sync Config Files Successfully.")
			return data.Body(), nil
		})
		if err != nil {
			ErrorHandle(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
