package goods

import (
	"testing"

	"github.com/xbpk3t/docs-alfred/internal/gh/model/consistency"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
)

// The generated goods model must stay in lockstep with goods.schema.json.
// Run `task gen` (go generate ./internal/gh/model/...) after changing the
// schema.

func TestGoodsModel_MatchesSchema(t *testing.T) {
	consistency.CheckDefs(t, schema.Goods, map[string]any{
		"topic":     Topic{},
		"record":    Record{},
		"tableItem": TableItem{},
	})
}

// The using view model (using.schema.json output shape) must stay in lockstep
// with the view schema too. Run `task gen` after changing either schema.
func TestUsingModel_MatchesSchema(t *testing.T) {
	consistency.CheckDefs(t, schema.Using, map[string]any{
		"usingType":  UsingType{},
		"usingTopic": UsingTopic{},
		"usingItem":  UsingItem{},
	})
}
