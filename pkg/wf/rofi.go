package wf

import (
	"encoding/json"
	"strings"
)

// RofiFormatter formats output for Rofi launcher.
type RofiFormatter struct{}

// Format formats the data for Rofi.
func (f *RofiFormatter) Format(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case []string:
		return strings.Join(v, "\n"), nil
	case []AlfredItem:
		// Convert Alfred items to Rofi format
		var lines []string
		for _, item := range v {
			lines = append(lines, item.Title)
		}

		return strings.Join(lines, "\n"), nil
	default:
		// Fallback to JSON
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}

		return string(bytes), nil
	}
}
