package goods

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

func TestGoods_StructV2(t *testing.T) {
	g := Goods{
		Type: "EDC",
		Tag:  "goods",
		Topics: []content.Topic{{
			Topic: "backpack",
			Score: 5,
			Table: []map[string]interface{}{{"name": "TYP7", "price": "¥339"}},
		}},
	}
	assert.Equal(t, "EDC", g.Type)
	assert.Equal(t, "goods", g.Tag)
	assert.Len(t, g.Topics, 1)
	assert.Equal(t, "backpack", g.Topics[0].Topic)
	assert.Equal(t, "¥339", g.Topics[0].Table[0]["price"])
}
