package gh

import (
	"testing"

	"github.com/xbpk3t/docs-alfred/internal/gh/model/consistency"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
)

// The generated gh model must stay in lockstep with gh.schema.json.
// Run `task gen` (go generate ./internal/gh/model/...) after changing the
// schema. tableItem is a free-form map in the schema, so it has no properties
// to compare against struct fields.

func TestGhModel_MatchesSchema(t *testing.T) {
	consistency.CheckDefs(t, schema.Gh, map[string]any{
		"topic":     Topic{},
		"repo":      Repo{},
		"record":    Record{},
		"tableItem": TableItem{},
	})
}
