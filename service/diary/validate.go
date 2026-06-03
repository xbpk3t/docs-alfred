package diary

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Validate checks Entry fields and returns validation issues.
func (e *Entry) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckScore(e.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(e.Date, "date", "date"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	return issues
}
