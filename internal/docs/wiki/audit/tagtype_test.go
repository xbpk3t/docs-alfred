package audit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keyOf(row StatRow, i int) string { return fmt.Sprintf("%v", row[i].Key) }
func valOf(row StatRow, i int) string { return fmt.Sprintf("%v", row[i].Value) }

func TestRunTagTypeStats_SingleTable_ShapeAndOrder(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"AI/x/a.md": researchFM, // AI/x: 2
		"AI/x/b.md": researchFM,
		"AI/z/c.md": researchFM, // AI/z: 1  → AI total 3
		"dev/D/d.md": "---\ntype: blog\ntitle: t\ndate: 2026-01-01\nsource: s\n---\n\nx\n", // dev/D: 0
		".hidden/h.md": "# ignored\n", // dot dir never a column
	})
	sections, err := RunTagTypeStats(root, 7)
	require.NoError(t, err)
	require.Len(t, sections, 1) // a SINGLE section/table
	sec := sections[0]
	assert.Equal(t, "research tag·type", sec.Section)
	require.Len(t, sec.Data, 2) // AI(total 3) then dev(total 0)

	ai := sec.Data[0]
	assert.Equal(t, "AI", valOf(ai, 0))
	assert.Equal(t, "类型1", keyOf(ai, 1))
	assert.Equal(t, "x (2)", valOf(ai, 1)) // top type first
	assert.Equal(t, "类型2", keyOf(ai, 2))
	assert.Equal(t, "z (1)", valOf(ai, 2))
	assert.Equal(t, "", valOf(ai, 3)) // no more types → empty
	// Every row exposes the full "<tag> + 类型1..7" header set.
	assert.Len(t, ai, 8)

	dev := sec.Data[1]
	assert.Equal(t, "dev", valOf(dev, 0))
	assert.Equal(t, "D (0)", valOf(dev, 1)) // 0-research type still a column
}

func TestRunTagTypeStats_ColumnCap(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"t/a/1.md": researchFM, "t/b/1.md": researchFM, "t/c/1.md": researchFM,
		"t/d/1.md": researchFM, "t/e/1.md": researchFM,
	})
	sections, err := RunTagTypeStats(root, 2) // cap to 2 type columns
	require.NoError(t, err)
	require.Len(t, sections, 1)
	row := sections[0].Data[0]
	require.Len(t, row, 3) // tag + 类型1 + 类型2
	assert.Equal(t, "a (1)", valOf(row, 1))
	assert.Equal(t, "b (1)", valOf(row, 2))
	// c,d,e exceed the cap → not shown.
}

func TestRunTagTypeStats_RowsOrderedByTotalDesc(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"big/t1/a.md": researchFM, "big/t1/b.md": researchFM, // big total 2
		"sm/t1/a.md": researchFM, // sm total 1
	})
	sections, err := RunTagTypeStats(root, 3)
	require.NoError(t, err)
	data := sections[0].Data
	require.Len(t, data, 2)
	assert.Equal(t, "big", valOf(data[0], 0)) // higher total first
	assert.Equal(t, "sm", valOf(data[1], 0))
}

func TestRunTagTypeStats_DefaultLimitSeven(t *testing.T) {
	root := makeWiki(t, map[string]string{
		"t/a/1.md": researchFM, "t/b/1.md": researchFM, "t/c/1.md": researchFM,
		"t/d/1.md": researchFM, "t/e/1.md": researchFM, "t/f/1.md": researchFM,
		"t/g/1.md": researchFM, "t/h/1.md": researchFM,
	})
	sections, err := RunTagTypeStats(root, 0) // 0 → default 7
	require.NoError(t, err)
	row := sections[0].Data[0]
	// "type1..类型7" header, a..g shown; h is the 8th type and exceeds the cap.
	assert.Equal(t, "类型7", keyOf(row, 7))
	assert.Equal(t, "g (1)", valOf(row, 7))
}
