package fileutil

import (
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

const appCacheDir = "docs-alfred"

// CachePath returns a platform-appropriate cache path under the docs-alfred
// application directory. Absolute inputs are returned unchanged.
func CachePath(rel string) string {
	if rel == "" || filepath.IsAbs(rel) {
		return rel
	}

	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "." {
		return filepath.Join(xdg.CacheHome, appCacheDir)
	}

	return filepath.Join(xdg.CacheHome, appCacheDir, cleaned)
}

// LegacyCachePath returns the historical repository-local cache path used by
// older versions of the tools.
func LegacyCachePath(rel string) string {
	cleaned := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(rel)), string(filepath.Separator))

	return filepath.Join(".cache", cleaned)
}
