package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type WranglerConfig struct {
	D1Databases []D1Database `toml:"d1_databases"`
}

type D1Database struct {
	Binding      string `toml:"binding"`
	DatabaseName string `toml:"database_name"`
	DatabaseID   string `toml:"database_id"`
}

func D1DatabaseID(path, binding string) (string, error) {
	var cfg WranglerConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return "", err
	}
	for _, db := range cfg.D1Databases {
		if db.Binding == binding {
			if db.DatabaseID == "" {
				return "", fmt.Errorf("binding %s has empty database_id", binding)
			}
			if strings.Contains(db.DatabaseID, "TODO") {
				return "", fmt.Errorf("binding %s has placeholder database_id", binding)
			}

			return db.DatabaseID, nil
		}
	}

	return "", fmt.Errorf("binding %s not found in %s", binding, path)
}
