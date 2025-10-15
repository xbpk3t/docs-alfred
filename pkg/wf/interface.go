package wf

// Formatter defines the interface for output formatters.
type Formatter interface {
	// Format formats the given data and returns a string representation
	Format(data interface{}) (string, error)
}

// GetFormatter returns the appropriate formatter based on the format string.
func GetFormatter(format string) Formatter {
	switch format {
	case "alfred":
		return &AlfredFormatter{}
	case "raw":
		return &RawFormatter{}
	case "rofi":
		return &RofiFormatter{}
	default:
		return &PlainFormatter{}
	}
}
