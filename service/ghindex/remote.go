package ghindex

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const (
	DefaultConfigURL = "https://cdn.lucc.dev/gh.yml"
	DefaultMaxAge    = 24 * time.Hour
)

var (
	DefaultConfigPath     = fileutil.CachePath("gh-alfred-gh.yml")
	backgroundSyncStarter = startBackgroundSyncProcess
)

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
	configURL = normalizeConfigURL(configURL)

	return &Manager{
		configPath: configPath,
		configURL:  configURL,
		maxAge:     DefaultMaxAge,
	}
}

func normalizeConfigURL(configURL string) string {
	if strings.HasSuffix(configURL, "/") {
		return configURL + "gh.yml"
	}

	return configURL
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
	data, err := httputil.Get(httputil.NewClient(httputil.DefaultClientTimeout), m.configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download config: %w", err)
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

// LoadWithBackgroundSync loads config from cache immediately.
// If the cache is stale, it triggers a background sync process (non-blocking).
// If no cache exists, it falls back to blocking LoadWithCacheTTL.
func (m *Manager) LoadWithBackgroundSync() error {
	info, err := os.Stat(m.configPath)
	if os.IsNotExist(err) {
		return m.LoadWithCacheTTL()
	}

	if time.Since(info.ModTime()) > m.maxAge {
		if err := backgroundSyncStarter(m); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: background sync unavailable, using cached config: %v\n", err)
		}
	}

	return m.loadFromFile()
}

func startBackgroundSyncProcess(m *Manager) error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current executable: %w", err)
	}

	cmd := exec.Command(binaryPath, "sync", "--url", m.configURL, "--cache", m.configPath)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}

	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	if err := cmd.Start(); err != nil {
		_ = devNull.Close()

		return fmt.Errorf("start background sync: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		_ = devNull.Close()

		return fmt.Errorf("release background sync process: %w", err)
	}
	if err := devNull.Close(); err != nil {
		return fmt.Errorf("close %s: %w", os.DevNull, err)
	}

	return nil
}

// IsBackgroundSyncAvailable checks if the binary can run background sync.
func IsBackgroundSyncAvailable(binaryPath string) bool {
	_, err := exec.LookPath(binaryPath)

	return err == nil
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
