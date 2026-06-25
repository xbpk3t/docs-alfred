package ghindex

import (
	"fmt"
	"log/slog"
	"os"

	yaml "github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

const (
	// LocalGHYMLPath is the default local path for gh.yml.
	LocalGHYMLPath = "/tmp/gh.yml"

	localSourceDir = "data/gh"
)

// LocalGHConfig configures local gh.yml loading.
type LocalGHConfig struct {
	// Path is the local gh.yml file path. Defaults to LocalGHYMLPath.
	Path string
	// SourceDir is the source data directory for lazy generation.
	// Defaults to "data/gh" (relative to working directory).
	SourceDir string
}

// LoadLocalGHYML loads ConfigRepos from a local gh.yml file.
// If the file does not exist and SourceDir is provided, it attempts
// lazy generation from the source directory.
func LoadLocalGHYML(cfg LocalGHConfig) (ConfigRepos, error) {
	path := lo.Ternary(cfg.Path != "", cfg.Path, LocalGHYMLPath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lazyGenerate(cfg, path)
		}

		return nil, err
	}

	return unmarshalConfig(data, path)
}

func lazyGenerate(cfg LocalGHConfig, outPath string) (ConfigRepos, error) {
	srcDir := lo.Ternary(cfg.SourceDir != "", cfg.SourceDir, localSourceDir)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("local gh.yml not found at %s and source dir %s does not exist; run `task data` to generate", outPath, srcDir)
	}

	slog.Info("local gh.yml not found, generating from source", "src", srcDir, "out", outPath)

	if _, err := WriteConfigYAMLFromDir(srcDir, outPath); err != nil {
		return nil, fmt.Errorf("lazy generate gh.yml from %s: %w", srcDir, err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read generated gh.yml %s: %w", outPath, err)
	}

	return unmarshalConfig(data, outPath)
}

func unmarshalConfig(data []byte, path string) (ConfigRepos, error) {
	if err := ValidateConfigYAML(data); err != nil {
		return nil, fmt.Errorf("invalid local gh.yml %s: %w", path, err)
	}

	var configRepos ConfigRepos
	if err := yaml.Unmarshal(data, &configRepos); err != nil {
		return nil, fmt.Errorf("parse local gh.yml %s: %w", path, err)
	}

	return configRepos, nil
}

// LocalTopicCatalog loads topic candidates from a local gh.yml file.
func LocalTopicCatalog(cfg LocalGHConfig) ([]TopicCandidate, error) {
	configRepos, err := LoadLocalGHYML(cfg)
	if err != nil {
		return nil, err
	}

	return configRepos.TopicCatalog(), nil
}
