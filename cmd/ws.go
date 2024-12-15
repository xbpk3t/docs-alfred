package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/alfred"
	"github.com/xbpk3t/docs-alfred/pkg/common"
	"github.com/xbpk3t/docs-alfred/pkg/ws"
)

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Workspace search command",
	Run:   handleWsCommand,
}

func handleWsCommand(cmd *cobra.Command, args []string) {
	builder := alfred.NewItemBuilder(wf)

	ws, err := ws.NewConfigWs(data)
	if err != nil {
		wf.FatalError(err)
	}

	tks := ws.SearchWs(args)

	for _, tk := range tks {
		item := builder.BuildBasicItem(
			tk.Name,
			tk.Des,
			tk.URL,
			common.IconStar,
		)
		builder.AddCommonModifiers(item, tk.URL, tk.Des)
	}

	wf.SendFeedback()
}
