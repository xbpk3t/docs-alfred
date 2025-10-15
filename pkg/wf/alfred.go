package wf

import (
	"encoding/json"
)

// AlfredItem represents an Alfred wf item.
type AlfredItem struct {
	Icon         *AlfredIcon           `json:"icon,omitempty"`
	Variables    map[string]string     `json:"variables,omitempty"`
	Mods         map[string]*AlfredMod `json:"mods,omitempty"`
	Text         *AlfredText           `json:"text,omitempty"`
	Title        string                `json:"title"`
	Subtitle     string                `json:"subtitle,omitempty"`
	Arg          string                `json:"arg,omitempty"`
	Autocomplete string                `json:"autocomplete,omitempty"`
	Valid        bool                  `json:"valid"`
}

// AlfredIcon represents an Alfred item icon.
type AlfredIcon struct {
	Path string `json:"path,omitempty"`
	Type string `json:"type,omitempty"`
}

// AlfredMod represents an Alfred item modifier.
type AlfredMod struct {
	Variables map[string]string `json:"variables,omitempty"`
	Arg       string            `json:"arg,omitempty"`
	Subtitle  string            `json:"subtitle,omitempty"`
	Valid     bool              `json:"valid"`
}

// AlfredText represents Alfred item text.
type AlfredText struct {
	Copy      string `json:"copy,omitempty"`
	Largetype string `json:"largetype,omitempty"`
}

// AlfredOutput represents the complete Alfred wf output.
type AlfredOutput struct {
	Items []AlfredItem `json:"items"`
}

// AlfredFormatter formats output for Alfred wf.
type AlfredFormatter struct{}

// Format formats the data as Alfred JSON.
func (f *AlfredFormatter) Format(data interface{}) (string, error) {
	var output AlfredOutput

	switch v := data.(type) {
	case string:
		// Simple string output
		output.Items = []AlfredItem{
			{
				Title: v,
				Arg:   v,
				Valid: true,
			},
		}
	case []AlfredItem:
		output.Items = v
	case AlfredOutput:
		output = v
	default:
		// Try to convert to JSON and use as title
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		output.Items = []AlfredItem{
			{
				Title: string(bytes),
				Valid: false,
			},
		}
	}

	bytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
