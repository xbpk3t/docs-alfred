package wf

import (
	"encoding/json"
)

// RawFormatter formats output as raw JSON.
type RawFormatter struct{}

// Format formats the data as raw JSON.
func (f *RawFormatter) Format(data interface{}) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
