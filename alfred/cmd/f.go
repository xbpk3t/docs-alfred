package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/xbpk3t/docs-alfred/pkg"

	"github.com/spf13/cobra"
)

var data []byte

const (
	ConfigGithub = "gh.yml"
)

var fCmd = &cobra.Command{
	Use:              "f",
	Short:            "Root search command",
	PersistentPreRun: handlePreRun,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("f called")
	},
}

func handlePreRun(cmd *cobra.Command, args []string) {
	if !wf.Cache.Exists(cfgFile) {
		ErrorHandle(&pkg.DocsAlfredError{Err: pkg.ErrConfigNotFound})
	}

	data, _ = wf.Cache.Load(cfgFile)

	if !wf.IsRunning(SyncJob) {
		cmd := exec.Command("./exe", SyncJob, fmt.Sprintf("--config=%s", cfgFile))
		slog.Info("sync cmd: ", slog.Any("cmd", cmd.String()))
		if err := wf.RunInBackground(SyncJob, cmd); err != nil {
			ErrorHandle(err)
		}
	}
}

func init() {
	// cfgManager = config.NewManager(wf, ConfigGithub)
	rootCmd.AddCommand(fCmd)
	fCmd.AddCommand(ghCmd)
	fCmd.AddCommand(wsCmd)
}
