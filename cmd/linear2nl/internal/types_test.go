package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupCount(t *testing.T) {
	assert.Equal(t, 0, GroupView{}.GroupCount())
	assert.Equal(t, 1, GroupView{Issues: []GroupItemView{{Identifier: "A"}}}.GroupCount())
	assert.Equal(t, 3, GroupView{Issues: []GroupItemView{{}, {}, {}}}.GroupCount())
}
