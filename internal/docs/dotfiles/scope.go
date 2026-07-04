package dotfiles

import (
	"path/filepath"
	"strings"
)

// scopeRule maps a directory prefix to a category extraction strategy.
type scopeRule struct {
	prefix string
	label  string
	idx    int
}

var scopeMap = []scopeRule{
	{prefix: "home/base", idx: 2},
	{prefix: "home/core", idx: 2},
	{prefix: "modules/nixos", idx: 2},
	{prefix: "modules/darwin", label: "desktop"},
	{prefix: "home/darwin", label: "desktop"},
	{prefix: "home/nixos", label: "desktop"},
}

// DefaultScope returns the default directory scopes for dotfiles scanning.
func DefaultScope() []string {
	scopes := make([]string, len(scopeMap))
	for i, r := range scopeMap {
		scopes[i] = r.prefix
	}
	return scopes
}

// categoryFromFile extracts the category name from a relative file path.
// Returns "" if the path doesn't match any known scope pattern.
func categoryFromFile(relPath string) string {
	relPath = filepath.Clean(relPath)
	for _, rule := range scopeMap {
		prefix := rule.prefix + string(filepath.Separator)
		if !strings.HasPrefix(relPath, prefix) {
			continue
		}
		rest := relPath[len(prefix):]
		if rule.label != "" {
			return rule.label
		}
		parts := strings.SplitN(rest, string(filepath.Separator), 2)
		if len(parts) >= 1 && parts[0] != "" {
			return parts[0]
		}
	}
	return ""
}
