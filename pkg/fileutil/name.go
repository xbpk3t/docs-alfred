package fileutil

import (
	"path/filepath"
	"strings"
)

// TypeFromFilename derives a data type tag from a YAML file name by stripping
// the extension, a leading dot-prefix (hierarchical files like .PHP.yml → "PHP"),
// and any supplied domain prefixes (e.g. "goods." so goods.EDC.yml → "EDC"). A
// name that is already an extensionless stem is returned as-is.
func TypeFromFilename(name string, prefixes ...string) string {
	base := filepath.Base(name)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	stem = strings.TrimPrefix(stem, ".")
	for _, p := range prefixes {
		stem = strings.TrimPrefix(stem, p)
	}

	return strings.TrimSpace(stem)
}
