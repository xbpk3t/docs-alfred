package fileutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadJSONFile reads path and unmarshals its JSON content into T.
func ReadJSONFile[T any](path string) (T, error) {
	var value T

	data, err := os.ReadFile(path)
	if err != nil {
		return value, fmt.Errorf("read json %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("parse json %s: %w", path, err)
	}

	return value, nil
}

// UnmarshalJSON unmarshals JSON bytes into T.
func UnmarshalJSON[T any](data []byte) (T, error) {
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return value, err
	}

	return value, nil
}

// MarshalJSON marshals value as indented JSON using the repository's standard
// two-space formatting.
func MarshalJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

// AtomicWriteJSONFile marshals value as indented JSON and atomically writes it.
func AtomicWriteJSONFile(path string, value any, perm os.FileMode) error {
	data, err := MarshalJSON(value)
	if err != nil {
		return fmt.Errorf("marshal json %s: %w", path, err)
	}
	if err := AtomicWriteFile(path, data, perm); err != nil {
		return fmt.Errorf("write json %s: %w", path, err)
	}

	return nil
}
