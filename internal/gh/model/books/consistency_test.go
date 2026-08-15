package books

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
)

// The generated books model must stay in lockstep with books.schema.json.
// Run `go generate ./internal/gh/model/books` after changing the schema.
// All $defs are covered — including tableItem, which the goods model test
// historically missed.

func TestBooksModel_MatchesSchema(t *testing.T) {
	defs := parseSchemaDefs(t, schema.Books)

	instances := map[string]any{
		"section":   Section{},
		"topic":     Topic{},
		"record":    Record{},
		"tableItem": TableItem{},
	}
	for name, inst := range instances {
		schemaProps := defs[name]
		structTags := yamlTags(reflect.TypeOf(inst))
		assert.ElementsMatchf(t, schemaProps, structTags, "def %s: schema properties vs model yaml tags", name)
	}
}

func parseSchemaDefs(t *testing.T, raw []byte) map[string][]string {
	t.Helper()
	var doc struct {
		Defs map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))

	out := map[string][]string{}
	for name, def := range doc.Defs {
		keys := make([]string, 0, len(def.Properties))
		for k := range def.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out[name] = keys
	}

	return out
}

func yamlTags(tp reflect.Type) []string {
	var tags []string
	for i := 0; i < tp.NumField(); i++ {
		tag := tp.Field(i).Tag.Get("yaml")
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" || name == "-" {
			continue
		}
		tags = append(tags, name)
	}
	sort.Strings(tags)

	return tags
}
