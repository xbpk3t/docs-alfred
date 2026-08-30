package wf

// Formatter defines the interface for output formatters.
type Formatter interface {
	// Format formats the given data and returns a string representation
	Format(data any) (string, error)
}

// Compile-time interface assertions.
var (
	_ Formatter = (*AlfredFormatter)(nil)
	_ Formatter = (*PlainFormatter)(nil)
	_ Formatter = (*RawFormatter)(nil)
)

// GetFormatter returns the appropriate formatter based on the format string.
func GetFormatter(format string) Formatter {
	switch format {
	case "alfred":
		return &AlfredFormatter{}
	case "raw", "json":
		return &RawFormatter{}
	default:
		return &PlainFormatter{}
	}
}
