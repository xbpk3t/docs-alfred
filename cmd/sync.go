package cmd

import (
	"errors"
	"log/slog"
	"time"

	"github.com/hxhac/docs-alfred/pkg/gh"
	"github.com/hxhac/docs-alfred/utils"
	"github.com/spf13/cobra"
)

const SyncJob = "sync"

const expire = 60

const (
	// KeyGithubAPIToken /* #nosec */
	KeyGithubAPIToken = "github-api-token"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		url := wf.Config.GetString("url") + cfgFile

		if url == "" {
			slog.Info("URL is Empty", slog.Any("URL", url))
			ErrorHandle(errors.New("URL is Empty"))
		}

		_, err := wf.Cache.LoadOrStore(cfgFile, time.Duration(expire)*time.Minute, func() ([]byte, error) {
			data, _ = utils.Fetch(url)

			switch cfgFile {
			case ConfigGithub:
				token, err := wf.Keychain.Get(KeyGithubAPIToken)
				if err != nil {
					slog.Error("get github token error", slog.Any("Error", err))
					return nil, err
				}
				gh := gh.NewRepos()
				if _, err := gh.UpdateRepositories(token, wf.CacheDir()+RepoDB); err != nil {
					slog.Error("failed to update repo by token", slog.Any("Error", err))
					ErrorHandle(err)
				}
			}
			return data, nil
		})
		if err != nil {
			ErrorHandle(err)
		}
		slog.Info("Sync Repos Successfully.")
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
