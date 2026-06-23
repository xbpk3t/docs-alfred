package goods

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoods_Struct(t *testing.T) {
	g := Goods{
		Tag:   "EDC",
		Type:  "耳机",
		Des:   "test",
		Score: 4,
	}
	assert.Equal(t, "EDC", g.Tag)
	assert.Equal(t, "耳机", g.Type)
	assert.Equal(t, "test", g.Des)
	assert.Equal(t, 4, g.Score)
}

func TestItem_Struct(t *testing.T) {
	item := Item{
		Name:     "C50",
		Param:    "参数",
		Price:    "¥179",
		Date:     "2023-04-29",
		EndDate:  "2025-08-27",
		EndPrice: "¥20",
		Des:      "test",
		URL:      "https://example.com",
		Use:      true,
	}
	assert.Equal(t, "C50", item.Name)
	assert.Equal(t, "¥179", item.Price)
	assert.Equal(t, "2023-04-29", item.Date)
	assert.Equal(t, "2025-08-27", item.EndDate)
	assert.Equal(t, "¥20", item.EndPrice)
	assert.True(t, item.Use)
}
