package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/utils"
)

const (
	ConfigGithub = "gh.yml"
	RepoDB       = "/repo.db"
)

var fCmd = &cobra.Command{
	Use:   "f",
	Short: "A brief description of your command",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if !wf.Cache.Exists(cfgFile) {
			ErrorHandle(&utils.DocsAlfredError{Err: utils.ErrConfigNotFound})
		}

		data, _ = wf.Cache.Load(cfgFile)

		if !wf.IsRunning(SyncJob) {
			cmd := exec.Command("./exe", SyncJob, fmt.Sprintf("--config=%s", cfgFile))
			slog.Info("sync cmd: ", slog.Any("cmd", cmd.String()))
			if err := wf.RunInBackground(SyncJob, cmd); err != nil {
				ErrorHandle(err)
			}
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("f called")
	},
}

var data []byte

func init() {
	rootCmd.AddCommand(fCmd)
	fCmd.AddCommand(ghCmd)
	fCmd.AddCommand(wsCmd)
}
