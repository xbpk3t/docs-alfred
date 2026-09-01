package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	goods "github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

func newGoodsCmd(dataPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goods",
		Short: "Goods data commands",
	}

	cmd.AddCommand(newUsingCmd(dataPath))
	cmd.AddCommand(newGoodsDumpCmd(dataPath))

	return cmd
}

// goodsRunE builds the RunE shared by `goods using` and `goods dump`: resolve the
// domain path, log, delegate to extract (which owns its error wrap), and write
// JSON. The two subcommands differ only in extract + the slog message.
func goodsRunE(dataPath *string, extract func(path string) (any, error), msg string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		path, err := data.DomainDefaultPath(data.DomainGoods, *dataPath)
		if err != nil {
			return err
		}

		slog.Info(msg, "domain", data.DomainGoods, "path", path)

		result, err := extract(path)
		if err != nil {
			return err
		}

		return output.WriteJSON(result)
	}
}

// newGoodsDumpCmd registers `goods dump`, the counterpart of `dump gh`: it dumps
// every goods topic with full content (record/des/qs/table), grouped by type,
// with no isUsing filter. Audit prompts read per-item score/des/record here that
// `goods using` drops.
func newGoodsDumpCmd(dataPath *string) *cobra.Command {
	extract := func(path string) (any, error) {
		result, err := goods.ExtractAll(path)
		if err != nil {
			return nil, fmt.Errorf("extract goods all: %w", err)
		}
		return result, nil
	}

	return &cobra.Command{
		Use:   "dump",
		Short: "Dump all goods topics with full content, grouped by type",
		Long: `Load goods data from the goods domain and output every topic with its
full content, grouped by type -> topic. Unlike "using", rows are included
regardless of isUsing and no field is projected away (per-item record/score/des
are kept).

Output:
	[
	  {
	    "type": "durs",
	    "topics": [
	      { "topic": "收纳袋", "record": [...], "des": "...", "table": [...] }
	    ]
	  }
	]`,
		RunE: goodsRunE(dataPath, extract, "Dumping all goods"),
	}
}

func newUsingCmd(dataPath *string) *cobra.Command {
	extract := func(path string) (any, error) {
		result, err := goods.ExtractUsing(path)
		if err != nil {
			return nil, fmt.Errorf("extract goods using: %w", err)
		}
		return result, nil
	}

	return &cobra.Command{
		Use:   "using",
		Short: "Extract goods items in use (isUsing: true)",
		Long: `Load goods data from the goods domain and output items with isUsing: true.

Output is grouped by type → topic:
	[
	  {
	    "type": "耐用品",
	    "topics": [
	      {
	        "topic": "收纳袋",
	        "items": [
	          { "name": "...", "brand": "...", "price": "...", ... }
	        ]
	      }
	    ]
	  }
	]`,
		RunE: goodsRunE(dataPath, extract, "Extracting goods in use"),
	}
}
