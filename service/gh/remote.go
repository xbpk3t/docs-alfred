package gh

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	yaml "github.com/goccy/go-yaml"
)

const (
	DefaultConfigURL  = "https://docs.lucc.dev/gh.yml"
	DefaultConfigPath = "/tmp/docs-cli-gh.yml"
)

// Manager handles remote repository configuration fetching and caching.
type Manager struct {
	configPath string
	configURL  string
	repos      Repos
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
	}
}

// Sync downloads the configuration file from remote URL.
func (m *Manager) Sync() error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	resp, err := http.Get(m.configURL)
	if err != nil {
		return fmt.Errorf("failed to download config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download config: HTTP %d", resp.StatusCode)
	}

	tmpFile := m.configPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpFile)

		return fmt.Errorf("failed to write config: %w", err)
	}
	_ = out.Close()

	if err := os.Rename(tmpFile, m.configPath); err != nil {
		_ = os.Remove(tmpFile)

		return fmt.Errorf("failed to rename config: %w", err)
	}

	return nil
}

// Load loads the configuration file (auto-syncs if missing).
func (m *Manager) Load() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		if err := m.Sync(); err != nil {
			return fmt.Errorf("config file not found and sync failed: %w", err)
		}
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var configRepos ConfigRepos
	if err := yaml.Unmarshal(data, &configRepos); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.repos = configRepos.ToRepos()

	return nil
}

// GetRepos returns all repositories.
func (m *Manager) GetRepos() Repos {
	return m.repos
}

// Filter filters repositories by query.
func (m *Manager) Filter(query string) Repos {
	return m.repos.Filter(query)
}
