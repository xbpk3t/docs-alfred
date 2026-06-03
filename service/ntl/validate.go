package ntl

import (
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// Validate checks Jav fields and returns validation issues.
func (j *Jav) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckScore(j.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(j.PublishAt, "publishAt", "date"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	for i := range j.Sub {
		for _, iss := range j.Sub[i].Validate(file) {
			iss.Message = "sub[" + itoa(i) + "]: " + iss.Message
			issues = append(issues, iss)
		}
	}

	return issues
}

// Validate checks VG fields and returns validation issues.
func (v *VG) Validate(file string) []checkutil.Issue {
	var issues []checkutil.Issue

	if iss := checkutil.CheckRequired(v.Name, "name"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckScore(v.Score); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}
	if iss := checkutil.CheckDate(v.PublishAt, "publishAt", "year"); iss != nil {
		iss.File = file
		issues = append(issues, *iss)
	}

	for i := range v.Sub {
		for _, iss := range v.Sub[i].Validate(file) {
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
