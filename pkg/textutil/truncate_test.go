package textutil

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestTruncateUTF8KeepsChineseTextValid(t *testing.T) {
	result := TruncateUTF8("你好，世界", 5)

	require.True(t, utf8.ValidString(result))
	require.Equal(t, "你...", result)
}

func TestTruncateUTF8SanitizesInvalidInput(t *testing.T) {
	result := TruncateUTF8("ok\xffbad", 20)

	require.True(t, utf8.ValidString(result))
	require.Contains(t, result, "\uFFFD")
}
