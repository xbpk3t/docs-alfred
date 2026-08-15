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

// DateYear/SeriesHint 已移除：publishAt 年份校验迁到 books.schema.json，
// SeriesHint 在生产代码中无引用。
