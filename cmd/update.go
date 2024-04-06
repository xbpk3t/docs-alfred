package cmd

import (
	"log/slog"
	"os"
	"os/exec"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

var EnabledAutoUpdate = "false"

const updateJobName = "checkForUpdate"

func CheckForUpdate() {
	if wf.UpdateCheckDue() && !wf.IsRunning(updateJobName) {
		slog.Info("Running update check in background...")
		cmd := exec.Command(os.Args[0], "update")
		if err := wf.RunInBackground(updateJobName, cmd); err != nil {
			slog.Error("Error starting update check: %s", err)
		}
	}

	if wf.UpdateAvailable() {
		wf.Configure(aw.SuppressUIDs(true))
		wf.NewItem("An update is available!").
			Subtitle("⇥ or ↩ to install update").
			Valid(false).
			Autocomplete("workflow:update").
			Icon(&aw.Icon{Value: "update-available.png"})
	}
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update docs-alfred",
	Run: func(cmd *cobra.Command, args []string) {
		if EnabledAutoUpdate == "true" {
			wf.Configure(aw.TextErrors(true))
			slog.Info("Checking for updates...")
			if err := wf.CheckForUpdate(); err != nil {
				wf.FatalError(err)
			}
		} else {
			slog.Info("Auto update disabled")
		}
	},
}

func init() {
}
