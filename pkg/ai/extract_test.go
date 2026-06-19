package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractJSONBracesIgnoresBracesInsideStrings(t *testing.T) {
	raw := `prefix {not json} \n {"summary":"value with { braces } inside"} suffix`

	require.Equal(t, `{"summary":"value with { braces } inside"}`, ExtractJSONBraces(raw))
}

func TestExtractJSONBracesHandlesEscapedQuotes(t *testing.T) {
	raw := `{"text":"quoted \"{still string}\" value","ok":true}`

	require.Equal(t, raw, ExtractJSONBraces(raw))
}

func TestUnmarshalStrictJSONAcceptsMarkdownFence(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var got payload
	err := UnmarshalStrictJSON("```json\n{\"name\":\"demo\"}\n```", &got)

	require.NoError(t, err)
	require.Equal(t, "demo", got.Name)
}

func TestUnmarshalStrictJSONAcceptsWrappedJSONObject(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var got payload
	err := UnmarshalStrictJSON(`answer: {ignore me} final payload {"name":"demo"}`, &got)

	require.NoError(t, err)
	require.Equal(t, "demo", got.Name)
}

func TestUnmarshalStrictJSONReturnsErrorWhenNoJSONObject(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var got payload
	err := UnmarshalStrictJSON("not json", &got)

	require.Error(t, err)
}
