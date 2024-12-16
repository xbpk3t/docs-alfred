package config

import (
	"fmt"
	aw "github.com/deanishe/awgo"
)

type Manager struct {
	wf      *aw.Workflow
	cfgFile string
}

func NewManager(wf *aw.Workflow, cfgFile string) *Manager {
	return &Manager{
		wf:      wf,
		cfgFile: cfgFile,
	}
}

func (m *Manager) Load() ([]byte, error) {
	if !m.wf.Cache.Exists(m.cfgFile) {
		return nil, fmt.Errorf("config file not found: %s", m.cfgFile)
	}
	return m.wf.Cache.Load(m.cfgFile)
}
