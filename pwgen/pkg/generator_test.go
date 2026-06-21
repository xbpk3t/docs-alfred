package pwgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDeterministic(t *testing.T) {
	config := NewConfig("testsecret", 16, true, true, false)
	generator := NewGenerator(config)

	result1, err := generator.Generate("github.com")
	require.NoError(t, err)

	result2, err := generator.Generate("github.com")
	require.NoError(t, err)

	assert.Equal(t, result1, result2, "same inputs should produce same output")
}

func TestGenerateDifferentWebsites(t *testing.T) {
	config := NewConfig("testsecret", 16, true, true, false)
	generator := NewGenerator(config)

	result1, err := generator.Generate("github.com")
	require.NoError(t, err)

	result2, err := generator.Generate("google.com")
	require.NoError(t, err)

	assert.NotEqual(t, result1, result2, "different websites should produce different passwords")
}

func TestGenerateLength(t *testing.T) {
	config := NewConfig("testsecret", 16, true, true, false)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 16, len(result), "password should match requested length")
}

func TestGenerateLongerLength(t *testing.T) {
	config := NewConfig("testsecret", 32, true, true, false)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 32, len(result), "password should match requested length")
}

func TestGenerateWithPunctuation(t *testing.T) {
	config := NewConfig("testsecret", 16, false, true, true)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 16, len(result))

	hasPunctuation := false
	for _, c := range result {
		for _, p := range "~*-+()!@#$^&" {
			if c == p {
				hasPunctuation = true
				break
			}
		}
		if hasPunctuation {
			break
		}
	}
	assert.True(t, hasPunctuation, "password should contain at least one punctuation character")
}

func TestGenerateWithoutPunctuation(t *testing.T) {
	config := NewConfig("testsecret", 16, false, true, false)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 16, len(result))

	for _, c := range result {
		for _, p := range "~*-+()!@#$^&" {
			assert.NotEqual(t, c, p, "password should not contain punctuation when disabled")
		}
	}
}

func TestGenerateWithUppercase(t *testing.T) {
	config := NewConfig("testsecret", 16, true, true, false)
	generator := NewGenerator(config)

	result, err := generator.Generate("github.com")
	require.NoError(t, err)
	assert.Equal(t, 16, len(result))

	hasUpper := false
	for _, c := range result {
		if c >= 'A' && c <= 'Z' {
			hasUpper = true
			break
		}
	}
	assert.True(t, hasUpper, "password should contain at least one uppercase letter")
}
