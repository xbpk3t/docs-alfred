package music

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Validate checks Music fields and returns validation issues.
func (m *Music) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckRequired(m.Name, "name"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckScore(m.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(m.PublishAt, "publishAt", "year"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	return issues
}
