// This file is hand-maintained (not go:generate'd). It holds filename-derived
// helpers for the Section model that the JSON Schema cannot express.
package gh

import "github.com/xbpk3t/docs-alfred/pkg/fileutil"

// TypeFromFilename derives a section's type tag from its data/gh YAML file
// name (e.g. algo.yml → "algo", infra.yml → "infra"). The new data/gh layout
// drops the top-level "type" key and infers it from the filename instead.
func TypeFromFilename(name string) string {
	return fileutil.TypeFromFilename(name)
}

// TypeString returns the section's type tag, or "" when it is unset.
func (s Section) TypeString() string {
	if s.Type == nil {
		return ""
	}

	return *s.Type
}
