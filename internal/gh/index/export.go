package ghindex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// LoadConfigReposFromDir renders split data/gh YAML files into remote Alfred records.
func LoadConfigReposFromDir(src string) (ConfigRepos, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("read gh dir error: %w", err)
	}

	var allRepos ConfigRepos
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repos, err := loadConfigReposFromTagDir(entry.Name(), filepath.Join(src, entry.Name()))
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
	}

	if len(allRepos) == 0 {
		return nil, errors.New("no gh data found in any subdirectory")
	}

	return allRepos, nil
}

// loadConfigReposFromTagDir reads each YAML file under a tag directory and
// yields one ConfigRepo per file (= one section). The new data/gh layout is a
// flat topic array with no top-level "type"/"topics" keys: the section type is
// derived from the file name (algo.yml→"algo") and the whole file becomes that
// section's topics. The tag is injected from the directory name.
func loadConfigReposFromTagDir(tag, dir string) (ConfigRepos, error) {
	files, err := fileutil.ListYAMLFilesRecursive(dir)
	if err != nil {
		return nil, fmt.Errorf("list gh subdir %s error: %w", tag, err)
	}

	var allRepos ConfigRepos
	for _, yf := range files {
		data, err := os.ReadFile(yf)
		if err != nil {
			return nil, fmt.Errorf("read gh %s error: %w", yf, err)
		}

		topics, err := parser.NewParser[gh.Topic](data).WithFileName(filepath.Base(yf)).ParseFlatten()
		if err != nil {
			return nil, fmt.Errorf("parse gh %s error: %w", yf, err)
		}
		if len(topics) == 0 {
			continue
		}

		cfg := &ConfigRepo{
			Tag:    tag,
			Type:   gh.TypeFromFilename(filepath.Base(yf)),
			Topics: topics,
		}
		normalizeConfigRepo(cfg)
		allRepos = append(allRepos, cfg)
	}

	return allRepos, nil
}

// MarshalConfigReposYAML serializes Alfred records to the remote gh.yml shape.
func MarshalConfigReposYAML(configRepos ConfigRepos) ([]byte, error) {
	data, err := yaml.Marshal(configRepos)
	if err != nil {
		return nil, fmt.Errorf("marshal gh repos error: %w", err)
	}

	return data, nil
}

// WriteConfigYAMLFromDir renders split data/gh YAML files and writes a gh.yml file.
func WriteConfigYAMLFromDir(src, out string) (int, error) {
	configRepos, err := LoadConfigReposFromDir(src)
	if err != nil {
		return 0, err
	}

	data, err := MarshalConfigReposYAML(configRepos)
	if err != nil {
		return 0, err
	}
	if err := ValidateConfigYAML(data); err != nil {
		return 0, err
	}

	if err := fileutil.AtomicWriteFile(out, data, fileutil.FilePermPrivate); err != nil {
		return 0, fmt.Errorf("write gh.yml: %w", err)
	}

	return len(configRepos.ToRepos()), nil
}

// ValidateConfigYAML verifies that bytes are parseable and useful as an Alfred index.
func ValidateConfigYAML(data []byte) error {
	var configRepos ConfigRepos
	if err := yaml.Unmarshal(data, &configRepos); err != nil {
		return fmt.Errorf("parse gh.yml: %w", err)
	}
	if len(configRepos) == 0 {
		return errors.New("gh.yml has no config entries")
	}
	if len(configRepos.ToRepos()) == 0 {
		return errors.New("gh.yml has no valid GitHub repositories")
	}

	return nil
}

// ValidateConfigYAMLFile verifies a gh.yml file on disk.
func ValidateConfigYAMLFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read gh.yml: %w", err)
	}

	return ValidateConfigYAML(data)
}
