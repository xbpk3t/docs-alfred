package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"

	"github.com/xbpk3t/docs-alfred/pkg"

	"github.com/deanishe/awgo/update"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

var (
	repo = "xbpk3t/docs-alfred"
	wf   *aw.Workflow
	av   = aw.NewArgVars()
)

// ErrorHandle handle error
func ErrorHandle(err error) {
	av.Var("error", err.Error())
	if err := av.Send(); err != nil {
		wf.Fatalf("failed to send args to Alfred: %v", err)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "docs-alfred",
	Run: func(cmd *cobra.Command, args []string) {
		wf.SendFeedback()
	},
}

var data []byte

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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	wf.Run(func() {
		if err := rootCmd.Execute(); err != nil {
			log.Println(err)
			os.Exit(1)
		}
	})
}

var cfgFile string

func InitWorkflow() {
	if wf == nil {
		wf = aw.New(update.GitHub(repo), aw.HelpURL(repo+"/issues"))
		wf.Args() // magic for "workflow:update"
	}
}

func init() {
	InitWorkflow()
	rootCmd.AddCommand(ghCmd)
	rootCmd.AddCommand(wsCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "qs.yml", "Config File To Parse")
	// rootCmd.MarkPersistentFlagRequired("config")
}
