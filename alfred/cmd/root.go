package cmd

import (
	"log"
	"os"

	"github.com/deanishe/awgo/update"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

var (
	repo = "xbpk3t/docs-alfred"
	wf   *aw.Workflow
	av   = aw.NewArgVars()
)

func ResetWorkflow() {
	wf = nil
}

// ErrorHandle handle error
func ErrorHandle(err error) {
	av.Var("error", err.Error())
	if err := av.Send(); err != nil {
		wf.Fatalf("failed to send args to Alfred: %v", err)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "docs-alfred",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		wf.SendFeedback()
	},
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "qs.yml", "Config File To Parse")
	// rootCmd.MarkPersistentFlagRequired("config")
}
