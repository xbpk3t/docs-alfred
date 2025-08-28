package config

import (
	"fmt"

	aw "github.com/deanishe/awgo"
)

// Manager handles configuration management for Alfred workflows.
type Manager struct {
	wf      *aw.Workflow
	cfgFile string
}

// NewManager creates a new configuration manager.
func NewManager(wf *aw.Workflow, cfgFile string) *Manager {
	return &Manager{
		wf:      wf,
		cfgFile: cfgFile,
	}
}

// Load loads configuration data from the cache.
func (m *Manager) Load() ([]byte, error) {
	if !m.wf.Cache.Exists(m.cfgFile) {
		return nil, fmt.Errorf("config file not found: %s", m.cfgFile)
	}
	return m.wf.Cache.Load(m.cfgFile)
}
