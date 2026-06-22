// Package validator provides struct validation using gookit/validate.
//
// Usage:
//
//	import "github.com/xbpk3t/docs-alfred/pkg/validator"
//
//	type MyStruct struct {
//	    Name string `validate:"required|min_len:3"`
//	}
//
//	if err := validator.Struct(&MyStruct{Name: "ab"}); err != nil {
//	    // handle error
//	}
package validator

import (
	"fmt"
	"regexp"

	"github.com/gookit/validate"
)

var (
	qualityRE  = regexp.MustCompile(`^[1-5]/5$`)
	durationRE = regexp.MustCompile(`^\d{1,2}:\d{2}(:\d{2})?$`)
	dateYmdRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// Setup registers project-specific validators with gookit/validate.
// Must be called once per process, typically in Execute() or main():
//
//	import "github.com/xbpk3t/docs-alfred/pkg/validator"
//
//	func Execute() {
//	    validator.Setup()
//	    ...
//	}
func Setup() {
	validate.AddValidators(map[string]any{
		"quality": func(val string) bool {
			return qualityRE.MatchString(val)
		},
		"duration": func(val string) bool {
			return durationRE.MatchString(val)
		},
		"date_ymd": func(val string) bool {
			return dateYmdRE.MatchString(val)
		},
	})
}

// Struct validates s using gookit/validate and returns an error if validation fails.
func Struct(s any) error {
	v := validate.Struct(s)
	if !v.Validate() {
		return fmt.Errorf("validation failed: %s", v.Errors.String())
	}

	return nil
}

// StructE validates s and returns the raw Errors for programmatic inspection.
func StructE(s any) validate.Errors {
	return v(s).ValidateE()
}

func v(s any) *validate.Validation {
	return validate.Struct(s)
}
