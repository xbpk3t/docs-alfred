package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
)

const SyncJob = "sync"

// const expire = 12

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		// url := wf.Config.GetString("url") + cfgFile

		// wf.Cache.LoadOrStore(cfgFile, time.Duration(expire)*time.Minute, func() ([]byte, error) {
		//
		// 	if url != "" {
		// 		resp, err := http.Get(url)
		// 		if err != nil {
		// 			slog.Error("request error", slog.Any("err", err))
		// 			return
		// 		}
		// 		defer resp.Body.Close()
		//
		// 		data, err := io.ReadAll(resp.Body)
		// 		if err != nil {
		// 			return
		// 		}
		// 		err = wf.Cache.Store(cfgFile, data)
		// 		if err != nil {
		// 			return
		// 		}
		// 	}
		//
		// 	switch cfgFile {
		// 	case "gh.yml":
		// 		token := wf.Config.GetString("gh_token")
		// 		gh := gh.NewRepos()
		// 		if _, err := gh.UpdateRepositories(token, wf.CacheDir()+"/repo.db"); err != nil {
		// 			// wf.NewWarningItem("Sync Failed.", err.Error()).Valid(false).Title("Sync Failed.")
		// 			// wf.SendFeedback()
		// 			// slog.Error("Sync Failed.", slog.Any("err", err))
		// 			ErrorHandle(err)
		// 		}
		// 	default:
		//
		// 	}
		//
		// 	return nil, nil
		// })

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
