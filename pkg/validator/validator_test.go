package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	// Setup should not panic
	require.NotPanics(t, Setup)
}

type testStruct struct {
	Name string `validate:"required|min_len:3"`
}

func TestStructValid(t *testing.T) {
	Setup()
	err := Struct(&testStruct{Name: "hello"})
	assert.NoError(t, err)
}

func TestStructInvalid(t *testing.T) {
	Setup()
	err := Struct(&testStruct{Name: "ab"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestStructEmpty(t *testing.T) {
	Setup()
	err := Struct(&testStruct{Name: ""})
	require.Error(t, err)
}

func TestStructEValid(t *testing.T) {
	Setup()
	errs := StructE(&testStruct{Name: "hello"})
	assert.True(t, errs.Empty())
}

func TestStructEInvalid(t *testing.T) {
	Setup()
	errs := StructE(&testStruct{Name: "ab"})
	assert.False(t, errs.Empty())
}

// --- Custom validator tests ---

func TestQualityValidator(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid 1/5", value: "1/5", valid: true},
		{name: "valid 3/5", value: "3/5", valid: true},
		{name: "valid 5/5", value: "5/5", valid: true},
		{name: "invalid 0/5", value: "0/5", valid: false},
		{name: "invalid 6/5", value: "6/5", valid: false},
		{name: "invalid no slash", value: "3", valid: false},
		{name: "invalid text", value: "good", valid: false},
		{name: "invalid empty", value: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, qualityRE.MatchString(tt.value), tt.valid)
		})
	}
}

func TestDurationValidator(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "mm:ss", value: "5:30", valid: true},
		{name: "h:mm:ss", value: "1:30:00", valid: true},
		{name: "zero padded", value: "05:30", valid: true},
		{name: "invalid no colon", value: "530", valid: false},
		{name: "invalid text", value: "abc", valid: false},
		{name: "invalid empty", value: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, durationRE.MatchString(tt.value), tt.valid)
		})
	}
}

func TestDateYmdValidator(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid date", value: "2024-01-15", valid: true},
		{name: "valid date 2", value: "2023-12-31", valid: true},
		{name: "invalid no leading zero", value: "2024-1-5", valid: false},
		{name: "invalid slashes", value: "2024/01/15", valid: false},
		{name: "invalid text", value: "not-a-date", valid: false},
		{name: "invalid empty", value: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, dateYmdRE.MatchString(tt.value), tt.valid)
		})
	}
}

// --- Integration tests through Struct to exercise Setup() lambdas ---

type qualityStruct struct {
	Quality string `validate:"required|quality"`
}

func TestQualityThroughStruct(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid 3/5", value: "3/5", valid: true},
		{name: "invalid 0/5", value: "0/5", valid: false},
		{name: "invalid text", value: "good", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Struct(&qualityStruct{Quality: tt.value})
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

type durationStruct struct {
	Duration string `validate:"required|duration"`
}

func TestDurationThroughStruct(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid mm:ss", value: "5:30", valid: true},
		{name: "valid h:mm:ss", value: "1:30:00", valid: true},
		{name: "invalid text", value: "abc", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Struct(&durationStruct{Duration: tt.value})
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

type dateYmdStruct struct {
	Date string `validate:"required|date_ymd"`
}

func TestDateYmdThroughStruct(t *testing.T) {
	Setup()

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid date", value: "2024-01-15", valid: true},
		{name: "invalid date", value: "2024/01/15", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Struct(&dateYmdStruct{Date: tt.value})
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
