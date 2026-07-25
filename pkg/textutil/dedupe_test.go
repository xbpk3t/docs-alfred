package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollapseDuplicateBlocks_ExactDouble(t *testing.T) {
	block := "hello\n\nworld"
	in := block + "\n\n" + block
	assert.Equal(t, block, CollapseDuplicateBlocks(in))
}

func TestCollapseDuplicateBlocks_NoDup(t *testing.T) {
	in := "alpha\n\nbeta"
	assert.Equal(t, in, CollapseDuplicateBlocks(in))
}

func TestCollapseDuplicateBlocks_Empty(t *testing.T) {
	assert.Empty(t, CollapseDuplicateBlocks(""))
}

func TestFirstLineTitle(t *testing.T) {
	assert.Equal(t, "hello world", FirstLineTitle("\n\nhello world\nmore", 50))
	assert.Equal(t, "title", FirstLineTitle("## title\nbody", 50))
	assert.Equal(t, "abc", FirstLineTitle("abcdefghij", 3))
	assert.Empty(t, FirstLineTitle("   \n  ", 50))
}
