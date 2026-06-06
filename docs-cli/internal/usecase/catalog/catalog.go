package catalog

import (
	"fmt"
	"log/slog"
	"time"

	gh "github.com/xbpk3t/docs-alfred/service/gh"
)

// SearchInput holds input for catalog search.
type SearchInput struct {
	ConfigURL string
	CachePath string
	Query     string
	MaxAge    string
}

// SearchResult holds catalog search results.
type SearchResult struct {
	Repos gh.Repos
}

// RunSearch searches GitHub repositories from remote gh.yml.
func RunSearch(input SearchInput) (*SearchResult, error) {
	manager := gh.NewManager(input.CachePath, input.ConfigURL)

	if input.MaxAge != "" {
		d, err := time.ParseDuration(input.MaxAge)
		if err == nil {
			manager.SetTTL(d)
		}
	}

	if err := manager.LoadWithCacheTTL(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &SearchResult{Repos: manager.Filter(input.Query)}, nil
}

// SyncInput holds input for catalog sync.
type SyncInput struct {
	ConfigURL string
	CachePath string
}

// SyncResult holds catalog sync details.
type SyncResult struct {
	ConfigURL string
	CachePath string
}

// RunSync forces a refresh of the remote gh.yml cache.
func RunSync(input SyncInput) (*SyncResult, error) {
	configURL := input.ConfigURL
	if configURL == "" {
		configURL = gh.DefaultConfigURL
	}
	cachePath := input.CachePath
	if cachePath == "" {
		cachePath = gh.DefaultConfigPath
	}

	manager := gh.NewManager(cachePath, configURL)
	if err := manager.Sync(); err != nil {
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	slog.Info("Sync completed successfully")

	return &SyncResult{ConfigURL: configURL, CachePath: cachePath}, nil
}
