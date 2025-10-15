package wf

import (
	"fmt"
	"strings"
)

// PlainFormatter formats output as plain text.
type PlainFormatter struct{}

// Format formats the data as plain text.
func (f *PlainFormatter) Format(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		return v, nil
	case []string:
		return strings.Join(v, "\n"), nil
	case map[string]interface{}:
		var result strings.Builder
		for key, value := range v {
			result.WriteString(fmt.Sprintf("%s: %v\n", key, value))
		}

		return result.String(), nil
	default:
		return fmt.Sprintf("%v", data), nil
	}
}
