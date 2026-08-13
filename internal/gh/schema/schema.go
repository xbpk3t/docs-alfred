// Package schema embeds the gh JSON Schema so data-cli check works without a
// runtime file lookup. The same gh.schema.json is the physical artifact an
// editor's yaml-language-server can reference for completion on data/gh/**/*.yml.
package schema

import _ "embed"

// Gh is the JSON Schema for data/gh YAML files.
//
//go:embed gh.schema.json
var Gh []byte
