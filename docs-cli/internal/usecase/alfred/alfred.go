package alfred

import (
	"fmt"
	"log/slog"
	"time"

	gh "github.com/xbpk3t/docs-alfred/service/gh"
)

// SearchInput holds input for Alfred search.
type SearchInput struct {
	ConfigURL string
	CachePath string
	Query     string
	MaxAge    string
}

// SearchResult holds Alfred search results.
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

// SyncInput holds input for Alfred cache sync.
type SyncInput struct {
	ConfigURL string
	CachePath string
}

// SyncResult holds Alfred cache sync details.
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

// ExportInput holds input for generating a remote gh.yml Alfred artifact.
type ExportInput struct {
	Src string
	Out string
}

// ExportResult holds generated Alfred artifact details.
type ExportResult struct {
	OutputPath string
	RepoCount  int
}

// RunExport renders split data/gh YAML files into a validated gh.yml artifact.
func RunExport(input ExportInput) (*ExportResult, error) {
	src := input.Src
	if src == "" {
		src = "data/gh"
	}
	out := input.Out
	if out == "" {
		out = "gh.yml"
	}

	repoCount, err := gh.WriteConfigYAMLFromDir(src, out)
	if err != nil {
		return nil, fmt.Errorf("export gh.yml failed: %w", err)
	}

	return &ExportResult{OutputPath: out, RepoCount: repoCount}, nil
}

// ValidateInput holds input for validating a gh.yml Alfred artifact.
type ValidateInput struct {
	File string
}

// ValidateResult holds Alfred validation details.
type ValidateResult struct {
	File string
}

// RunValidate validates a gh.yml artifact can be consumed by Alfred search.
func RunValidate(input ValidateInput) (*ValidateResult, error) {
	file := input.File
	if file == "" {
		file = "gh.yml"
	}
	if err := gh.ValidateConfigYAMLFile(file); err != nil {
		return nil, fmt.Errorf("validate gh.yml failed: %w", err)
	}

	return &ValidateResult{File: file}, nil
}
