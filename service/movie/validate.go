package movie

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Validate checks Movie fields and returns validation issues.
func (m *Movie) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckRequired(m.Name, "name"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckScore(m.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(m.ReadAt, "readAt", "date"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(m.PublishAt, "publishAt", "year"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	for i := range m.Sub {
		for _, iss := range m.Sub[i].Validate(file) {
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
