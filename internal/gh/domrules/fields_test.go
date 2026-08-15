package domrules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// books/ntl/goods 走 schema 校验,结构化 check 只剩 diary → 只保留 DiaryFields 相关测试。

func TestDiaryFields(t *testing.T) {
	assert.True(t, DiaryFields["date"])
	assert.True(t, DiaryFields["review"])
	assert.True(t, DiaryFields["des"])
	assert.True(t, DiaryFields["score"])
	assert.True(t, DiaryFields["week"])
	assert.True(t, DiaryFields["url"])
	assert.False(t, DiaryFields["name"])
}

func TestForbiddenFields(t *testing.T) {
	assert.True(t, ForbiddenFields["category"])
	assert.False(t, ForbiddenFields["name"])
}

func TestDateYearPattern(t *testing.T) {
	assert.True(t, DateYear.MatchString("2024"))
	assert.True(t, DateYear.MatchString("0"))
	assert.True(t, DateYear.MatchString("-100"))
	assert.False(t, DateYear.MatchString("abcde"))
	assert.False(t, DateYear.MatchString("12345"))
}

func TestSeriesHintPattern(t *testing.T) {
	assert.True(t, SeriesHint.MatchString("三部曲"))
	assert.True(t, SeriesHint.MatchString("系列"))
	assert.True(t, SeriesHint.MatchString("四部曲"))
	assert.True(t, SeriesHint.MatchString("合集"))
	assert.False(t, SeriesHint.MatchString("single"))
}
