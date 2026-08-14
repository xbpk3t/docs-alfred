// Package schema embeds the gh and goods JSON Schemas so data-cli check works
// without a runtime file lookup. The same .schema.json files are the physical
// artifacts an editor's yaml-language-server can reference for completion on
// data/gh/**/*.yml and data/goods/*.yml.
package schema

import _ "embed"

// Gh is the JSON Schema for data/gh YAML files.
//
//go:embed gh.schema.json
var Gh []byte

// Goods is the JSON Schema for data/goods YAML files.
//
//go:embed goods.schema.json
var Goods []byte
