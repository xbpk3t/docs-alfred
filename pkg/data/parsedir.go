package data

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

// ParseYAMLDir reads all YAML files in a directory and attempts to parse them.
func ParseYAMLDir(path string) (int, []error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, []error{fmt.Errorf("read dir %s: %w", path, err)}
	}

	var errs []error
	count := 0
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != extYML && ext != extYAML {
			continue
		}

		filePath := filepath.Join(path, e.Name())
		if err := parseYAMLFile(filePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filePath, err))

			continue
		}
		count++
	}

	return count, errs
}

// parseYAMLFile reads and decodes a single YAML file.
func parseYAMLFile(filePath string) error {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	// Parse multi-document YAML
	decoder := yaml.NewDecoder(bytes.NewReader(fileData))
	docCount := 0
	for {
		var doc any
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("doc[%d]: YAML parse error: %w", docCount, err)
		}
		docCount++
	}

	return nil
}
