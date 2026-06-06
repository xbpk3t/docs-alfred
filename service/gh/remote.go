package gh

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

const (
	DefaultConfigURL = "https://cdn.lucc.dev/gh.yml"
	DefaultMaxAge    = 24 * time.Hour
)

var DefaultConfigPath = fileutil.CachePath("gh-alfred-gh.yml")

// Manager handles remote repository configuration fetching and caching.
type Manager struct {
	configPath string
	configURL  string
	repos      Repos
	maxAge     time.Duration
}

// NewManager creates a new repository manager.
func NewManager(configPath, configURL string) *Manager {
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	if configURL == "" {
		configURL = DefaultConfigURL
	}

	return &Manager{
		configPath: configPath,
		configURL:  configURL,
		maxAge:     DefaultMaxAge,
	}
}

// SetTTL overrides the default cache max-age.
func (m *Manager) SetTTL(ttl time.Duration) {
	m.maxAge = ttl
}

// Sync downloads and validates the configuration file from remote URL.
func (m *Manager) Sync() error {
	data, err := m.download()
	if err != nil {
		return err
	}
	if err := ValidateConfigYAML(data); err != nil {
		return fmt.Errorf("invalid remote config: %w", err)
	}

	return m.writeCache(data)
}

func (m *Manager) download() ([]byte, error) {
	resp, err := http.Get(m.configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download config: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read config response: %w", err)
	}

	return data, nil
}

func (m *Manager) writeCache(data []byte) error {
	if err := fileutil.AtomicWriteFile(m.configPath, data, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("failed to write config cache: %w", err)
	}

	return nil
}

// LoadWithCacheTTL loads config with cache TTL checking.
// If the cache is stale (older than maxAge), it tries to sync.
// If sync fails but cache exists, it uses the stale cache with a warning.
// If sync fails and no cache exists, it returns an error.
func (m *Manager) LoadWithCacheTTL() error {
	info, err := os.Stat(m.configPath)
	if os.IsNotExist(err) {
		// No cache at all: try sync
		if err := m.Sync(); err != nil {
			return fmt.Errorf("config not cached and sync failed: %w", err)
		}

		return m.loadFromFile()
	}

	// Cache exists: check TTL
	if time.Since(info.ModTime()) > m.maxAge {
		// Cache is stale: try to sync
		if syncErr := m.Sync(); syncErr != nil {
			// Sync failed: warn and use stale cache
			fmt.Fprintf(os.Stderr, "WARNING: cache refresh failed, using stale cache: %v\n", syncErr)
			if loadErr := m.loadFromFile(); loadErr != nil {
				return fmt.Errorf("stale cache also unreadable: %w", loadErr)
			}

			return nil
		}
		// Sync succeeded: load fresh data
		return m.loadFromFile()
	}

	// Cache is fresh: load from file. If the cache was polluted by a bad
	// remote response from an older version, ignore it and try to refresh.
	if err := m.loadFromFile(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cached config invalid, refreshing cache: %v\n", err)
		if syncErr := m.Sync(); syncErr != nil {
			return fmt.Errorf("cached config invalid and sync failed: %w", syncErr)
		}

		return m.loadFromFile()
	}

	return nil
}

func (m *Manager) loadFromFile() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := ValidateConfigYAML(data); err != nil {
		return fmt.Errorf("invalid cached config: %w", err)
	}

	var configRepos ConfigRepos
	if err := yaml.Unmarshal(data, &configRepos); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.repos = configRepos.ToRepos()

	return nil
}

// Filter filters repositories by query.
func (m *Manager) Filter(query string) Repos {
	return m.repos.Filter(query)
}
