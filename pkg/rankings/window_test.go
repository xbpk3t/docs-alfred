package rankings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAllTimeWindow(t *testing.T) {
	w := &AllTimeWindow{Name: "global"}
	assert.Equal(t, "global", w.CurrentKey(), "AllTimeWindow should return the fixed name")
}

func TestDailyWindow(t *testing.T) {
	cst := time.FixedZone("CST", 8*3600)
	w := NewDailyWindow(cst)

	key := w.CurrentKey()
	// key 应该是 YYYYMMDD 格式，8 位数字
	assert.Len(t, key, 8, "DailyWindow key should be 8 digits YYYYMMDD")

	// 同一实例连续调用应返回相同 key（同一天内窗口稳定）
	assert.Equal(t, key, w.CurrentKey(), "DailyWindow key should be stable within the same day")
}
