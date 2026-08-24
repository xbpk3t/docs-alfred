package books

import (
	"testing"

	"github.com/xbpk3t/docs-alfred/internal/gh/model/consistency"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
)

// The generated books model must stay in lockstep with books.schema.json.
// Run `task gen` (go generate ./internal/gh/model/...) after changing the
// schema. All $defs are covered — including tableItem, which the goods model
// test historically missed.

func TestBooksModel_MatchesSchema(t *testing.T) {
	consistency.CheckDefs(t, schema.Books, map[string]any{
		"topic":     Topic{},
		"record":    Record{},
		"tableItem": TableItem{},
	})
}
