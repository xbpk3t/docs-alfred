package cmd

import (
	"bytes"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/docs/pkg"
	pkgErr "github.com/xbpk3t/docs-alfred/pkg"

	"github.com/deanishe/awgo/update"
	yaml "github.com/goccy/go-yaml"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

const repo = "xbpk3t/docs-alfred"

// AppContext holds application-wide dependencies
type AppContext struct {
	wf *aw.Workflow
	av *aw.ArgVars
}

// newAppContext creates a new application context
func newAppContext() *AppContext {
	return &AppContext{
		av: aw.NewArgVars(),
	}
}

// ErrorHandle handle error
func (ctx *AppContext) ErrorHandle(err error) {
	ctx.av.Var("error", err.Error())
	if err := ctx.av.Send(); err != nil {
		ctx.wf.Fatalf("failed to send args to Alfred: %v", err)
	}
}

// createRootCmd creates the root command with the given context
func createRootCmd(ctx *AppContext, cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use: "docs-alfred",
		Run: func(_ *cobra.Command, args []string) {
			// read configs from file
			configData, err := os.ReadFile(*cfgFile)
			if err != nil {
				ctx.ErrorHandle(err)
				return
			}

			if err := ctx.handlePreRun(args, *cfgFile); err != nil {
				ctx.ErrorHandle(err)
				return
			}

			cmd := exec.Command("./exe", SyncJob, "--config="+filepath.Clean(*cfgFile))
			if err := cmd.Run(); err != nil {
				log.Printf("Error: %v", err)
			}

			config := &pkg.DocsConfig{}
			decoder := yaml.NewDecoder(bytes.NewReader(configData), yaml.UseJSONUnmarshaler())
			if err := decoder.Decode(config); err != nil {
				ctx.ErrorHandle(err)
				return
			}

			if err := config.Process(); err != nil {
				ctx.ErrorHandle(err)
				return
			}

			if err := ctx.av.Send(); err != nil {
				ctx.ErrorHandle(err)
			}
		},
	}
}

func (ctx *AppContext) handlePreRun(_ []string, cfgFile string) error {
	if !ctx.wf.Cache.Exists(cfgFile) {
		return &pkgErr.DocsAlfredError{Err: pkgErr.ErrConfigNotFound}
	}

	_, _ = ctx.wf.Cache.Load(cfgFile)

	if !ctx.wf.IsRunning(SyncJob) {
		cmd := exec.Command("./exe", SyncJob, "--config="+filepath.Clean(cfgFile))
		slog.Info("sync cmd: ", slog.Any("cmd", cmd.String()))
		if err := ctx.wf.RunInBackground(SyncJob, cmd); err != nil {
			return err
		}
	}
	return nil
}

// InitWorkflow initializes the workflow in the context
func (ctx *AppContext) InitWorkflow() {
	if ctx.wf == nil {
		ctx.wf = aw.New(update.GitHub(repo), aw.HelpURL(repo+"/issues"))
		ctx.wf.Args() // magic for "workflow:update"
	}
}

// Execute runs the root command
func Execute() {
	ctx := newAppContext()
	ctx.InitWorkflow()

	var cfgFile string
	rootCmd := createRootCmd(ctx, &cfgFile)
	rootCmd.AddCommand(createGhCmd())

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "qs.yml", "Config File To Parse")

	ctx.wf.Run(func() {
		if err := rootCmd.Execute(); err != nil {
			ctx.ErrorHandle(err)
		}
	})
}
