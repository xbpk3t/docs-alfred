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
	DefaultConfigPath = "/tmp/gh.yml"
)

// Manager handles repository configuration.
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
//
//nolint:revive // Complexity is necessary for proper error handling
func (m *Manager) Sync() error {
	// Create directory if needed
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download file
	resp, err := http.Get(m.configURL)
	if err != nil {
		return fmt.Errorf("failed to download config: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download config: HTTP %d", resp.StatusCode)
	}

	// Create temp file
	tmpFile := m.configPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close temp file: %w", closeErr)
		}
	}()

	// Copy content
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(tmpFile)

		return fmt.Errorf("failed to write config: %w", err)
	}

	// Rename temp file to final file
	if err := os.Rename(tmpFile, m.configPath); err != nil {
		_ = os.Remove(tmpFile)

		return fmt.Errorf("failed to rename config: %w", err)
	}

	return nil
}

// Load loads the configuration file.
func (m *Manager) Load() error {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Try to sync first
		if err := m.Sync(); err != nil {
			return fmt.Errorf("config file not found and sync failed: %w", err)
		}
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Parse YAML
	var configRepos ConfigRepos
	if err := yaml.Unmarshal(data, &configRepos); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Convert to flat repos list
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
