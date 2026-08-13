// Package schema embeds the prpt JSON Schema so skx check works without a
// runtime file lookup. The same file is the physical artifact the editor's
// yaml-language-server references for completion on references/**/*.yml.
package schema

import _ "embed"

// Prpt is the JSON Schema for zzz prompt YAML files (prpt.yml).
//
//go:embed prpt.schema.json
var Prpt []byte
