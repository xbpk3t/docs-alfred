package textutil

import (
	"strings"

	"github.com/gosimple/slug"
)

const fallbackSlug = "untitled"

// SlugFilename converts free-form text into a stable ASCII-safe filename stem.
func SlugFilename(value string) string {
	s := slug.Make(strings.TrimSpace(value))
	if s == "" {
		return fallbackSlug
	}

	return s
}
