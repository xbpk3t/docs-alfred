package goods

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

// Goods 定义 v2 商品配置结构（type/tag/topics），与 goods.*.yml 对齐。
// 旧 v1 生命周期字段（using/item/use/endDate/endPrice 顶层）已移除。
type Goods struct {
	Type   string          `json:"type"   yaml:"type"`
	Tag    string          `json:"tag"    yaml:"tag"`
	Topics []content.Topic `json:"topics" yaml:"topics,omitempty"`
}
