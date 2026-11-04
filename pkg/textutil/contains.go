package textutil

import "strings"

// ContainsAny reports whether s contains any of the marker substrings.
func ContainsAny(s string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(s, marker) {
			return true
		}
	}

	return false
}
