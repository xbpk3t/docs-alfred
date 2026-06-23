package domrules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentFields(t *testing.T) {
	assert.True(t, ContentFields["name"])
	assert.True(t, ContentFields["alias"])
	assert.True(t, ContentFields["author"])
	assert.True(t, ContentFields["score"])
	assert.True(t, ContentFields["readAt"])
	assert.True(t, ContentFields["publishAt"])
	assert.True(t, ContentFields["des"])
	assert.True(t, ContentFields["record"])
	assert.True(t, ContentFields["sub"])
	assert.True(t, ContentFields["item"])
	assert.True(t, ContentFields["tags"])
	assert.True(t, ContentFields["url"])
	assert.True(t, ContentFields["cast"])
	assert.True(t, ContentFields["dict"])
}

func TestMusicFields(t *testing.T) {
	assert.True(t, MusicFields["name"])
	assert.True(t, MusicFields["author"])
	assert.True(t, MusicFields["score"])
	assert.True(t, MusicFields["perf"])
	assert.True(t, MusicFields["label"])
	assert.True(t, MusicFields["conductor"])
	assert.False(t, MusicFields["alias"])
}

func TestDiaryFields(t *testing.T) {
	assert.True(t, DiaryFields["date"])
	assert.True(t, DiaryFields["review"])
	assert.True(t, DiaryFields["des"])
	assert.True(t, DiaryFields["score"])
	assert.True(t, DiaryFields["week"])
	assert.True(t, DiaryFields["url"])
	assert.False(t, DiaryFields["name"])
}

func TestJavFields(t *testing.T) {
	assert.True(t, JavFields["url"])
	assert.True(t, JavFields["cast"])
	assert.True(t, JavFields["score"])
	assert.True(t, JavFields["des"])
	assert.True(t, JavFields["tags"])
	assert.True(t, JavFields["record"])
	assert.True(t, JavFields["rel"])
	assert.True(t, JavFields["sub"])
	assert.True(t, JavFields["label"])
	assert.True(t, JavFields["publishAt"])
	assert.True(t, JavFields["name"])
}

func TestVGFields(t *testing.T) {
	assert.True(t, VGFields["name"])
	assert.True(t, VGFields["developer"])
	assert.True(t, VGFields["price"])
	assert.True(t, VGFields["des"])
	assert.True(t, VGFields["playAt"])
	assert.True(t, VGFields["score"])
	assert.True(t, VGFields["record"])
	assert.True(t, VGFields["tags"])
	assert.True(t, VGFields["url"])
	assert.True(t, VGFields["sub"])
	assert.True(t, VGFields["table"])
	assert.True(t, VGFields["genre"])
	assert.True(t, VGFields["status"])
	assert.True(t, VGFields["platform"])
	assert.True(t, VGFields["publishAt"])
	assert.True(t, VGFields["alias"])
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
