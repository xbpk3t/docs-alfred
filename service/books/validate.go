package books

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Validate checks Book fields and returns validation issues.
func (b *Book) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckRequired(b.Name, "name"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckScore(b.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(b.ReadAt, "readAt", "date"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(b.PublishAt, "publishAt", "year"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	// Recursively validate sub-items
	for i := range b.Sub {
		for _, iss := range b.Sub[i].Validate(file) {
			iss.Message = "sub[" + itoa(i) + "]: " + iss.Message
			issues = append(issues, iss)
		}
	}

	return issues
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}
