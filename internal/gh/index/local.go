package ghindex

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultSourceDir is the split data/gh tree used as the sole source for local topic catalogs.
const DefaultSourceDir = "data/gh"

// LocalGHConfig configures loading topics from the split data/gh directory.
type LocalGHConfig struct {
	// SourceDir is an explicit data/gh root. When empty, derived from WikiRoot or DefaultSourceDir.
	SourceDir string
	// WikiRoot is the wiki directory (…/docs/wiki). Used to resolve sibling …/docs/data/gh
	// when SourceDir is empty.
	WikiRoot string
}

// SourceDirFromWikiRoot returns <parent-of-wiki>/data/gh.
// Empty wikiRoot falls back to DefaultSourceDir (cwd-relative).
func SourceDirFromWikiRoot(wikiRoot string) string {
	wikiRoot = strings.TrimSpace(wikiRoot)
	if wikiRoot == "" {
		return DefaultSourceDir
	}
	abs, err := filepath.Abs(wikiRoot)
	if err != nil {
		abs = wikiRoot
	}

	return filepath.Join(filepath.Dir(abs), DefaultSourceDir)
}

func resolveSourceDir(cfg LocalGHConfig) string {
	if src := strings.TrimSpace(cfg.SourceDir); src != "" {
		return src
	}
	if wiki := strings.TrimSpace(cfg.WikiRoot); wiki != "" {
		return SourceDirFromWikiRoot(wiki)
	}

	return DefaultSourceDir
}

// LocalTopicCatalog loads formal topic candidates via LoadConfigReposFromDir + TopicCatalog.
// It always reads the split YAML tree; there is no /tmp/gh.yml cache path.
func LocalTopicCatalog(cfg LocalGHConfig) ([]TopicCandidate, error) {
	src := resolveSourceDir(cfg)

	repos, err := LoadConfigReposFromDir(src)
	if err != nil {
		return nil, fmt.Errorf("load gh topics from %s: %w", src, err)
	}

	return repos.TopicCatalog(), nil
}

// RemoteTopicCatalog loads formal topic candidates from a remote gh.yml URL,
// using the same cache+TTL semantics as gh-alfred's Manager. When url is empty
// it falls back to the local split tree via LocalTopicCatalog.
func RemoteTopicCatalog(cfg LocalGHConfig, url string) ([]TopicCandidate, error) {
	if strings.TrimSpace(url) == "" {
		return LocalTopicCatalog(cfg)
	}

	manager := NewManager(DefaultConfigPath, url)
	if err := manager.LoadWithBackgroundSync(); err != nil {
		return nil, fmt.Errorf("load remote gh topics from %s: %w", url, err)
	}

	return manager.ConfigRepos().TopicCatalog(), nil
}
