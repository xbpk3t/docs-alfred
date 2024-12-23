package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/alfred/internal/alfred"
	"github.com/xbpk3t/docs-alfred/alfred/internal/cons"
	"github.com/xbpk3t/docs-alfred/service/ws"
)

var wsCmd = &cobra.Command{
	Use:              "ws",
	Short:            "Workspace search command",
	PersistentPreRun: handlePreRun,
	Run:              handleWsCommand,
}

func handleWsCommand(cmd *cobra.Command, args []string) {
	builder := alfred.NewItemBuilder(wf)

	f, err := ws.ParseConfig(data)
	if err != nil {
		wf.FatalError(err)
	}

	tks := f.Search(args)

	for _, tk := range tks {
		item := builder.BuildBasicItem(
			tk.Name,
			tk.Des,
			tk.URL,
			cons.IconCheck,
		)
		builder.AddCommonModifiers(item, tk.URL, tk.Des)
	}

	wf.SendFeedback()
}
