package importer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/rules"
)

func TestRunWithAlipayFile(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "alipay.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易号,交易创建时间,类型,交易对方,商品名称,金额（元）,收/支,交易状态,商家订单号,备注
2026050100001,2026-05-01 10:00:00,消费,淘宝商店,衣服,100.00,支出,交易成功,,购物
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:  1,
		Defaults: rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		AlipayFiles: []string{csvPath},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Transactions)
	assert.Len(t, result.SourceFiles, 1)
}

func TestRunWithBothFiles(t *testing.T) {
	wechatPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(wechatPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-01 10:00:00,商户消费,麦当劳,汉堡,零钱,支出,¥35.00,支付成功,wx-trade-1,,午餐
`), 0600))

	alipayPath := filepath.Join(t.TempDir(), "alipay.csv")
	require.NoError(t, os.WriteFile(alipayPath, []byte(`交易号,交易创建时间,类型,交易对方,商品名称,金额（元）,收/支,交易状态,商家订单号,备注
2026050200001,2026-05-02 10:00:00,消费,淘宝商店,衣服,100.00,支出,交易成功,,购物
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:  1,
		Defaults: rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		WechatFiles: []string{wechatPath},
		AlipayFiles: []string{alipayPath},
	})
	require.NoError(t, err)
	assert.Len(t, result.Transactions, 2)
	assert.Len(t, result.SourceFiles, 2)
}

func TestRunWithInvalidWechatFile(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "bad.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte("invalid csv content"), 0600))

	_, err := Run(&Input{
		Now:         time.Now(),
		WechatFiles: []string{csvPath},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse wechat")
}

func TestRunWithInvalidAlipayFile(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "bad_alipay.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte("invalid csv content"), 0600))

	_, err := Run(&Input{
		Now:         time.Now(),
		AlipayFiles: []string{csvPath},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse alipay")
}

func TestRunWithNonexistentAlipayFile(t *testing.T) {
	_, err := Run(&Input{
		Now:         time.Now(),
		AlipayFiles: []string{"/nonexistent/alipay.csv"},
	})
	assert.Error(t, err)
}

func TestRunWithLimitZero(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-01 10:00:00,商户消费,A,商品1,零钱,支出,¥10.00,支付成功,wx-1,,
2026-05-02 10:00:00,商户消费,B,商品2,零钱,支出,¥20.00,支付成功,wx-2,,
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:  1,
		Defaults: rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		WechatFiles: []string{csvPath},
		Limit:       0,
	})
	require.NoError(t, err)
	assert.Len(t, result.Transactions, 2) // no limit
}

func TestRunWithSorting(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-03 10:00:00,商户消费,C,商品,零钱,支出,¥30.00,支付成功,wx-3,,
2026-05-01 10:00:00,商户消费,A,商品,零钱,支出,¥10.00,支付成功,wx-1,,
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:  1,
		Defaults: rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		WechatFiles: []string{csvPath},
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 2)
	assert.Equal(t, "wx-1", result.Transactions[0].SourceTradeNo)
	assert.Equal(t, "wx-3", result.Transactions[1].SourceTradeNo)
}

func TestRunWithNonexistentFile(t *testing.T) {
	_, err := Run(&Input{
		Now:         time.Now(),
		WechatFiles: []string{"/nonexistent/file.csv"},
	})
	assert.Error(t, err)
}

func TestRunSortTieBreakByID(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	// Two transactions with the exact same timestamp but different trade numbers.
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-01 10:00:00,商户消费,B,商品2,零钱,支出,¥20.00,支付成功,wx-2,,
2026-05-01 10:00:00,商户消费,A,商品1,零钱,支出,¥10.00,支付成功,wx-1,,
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:  1,
		Defaults: rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		WechatFiles: []string{csvPath},
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 2)
	// Same OccurredAt => tie-break by ID (source:sourceTradeNo).
	// wechat:wx-1 < wechat:wx-2
	assert.Equal(t, "wx-1", result.Transactions[0].SourceTradeNo)
	assert.Equal(t, "wx-2", result.Transactions[1].SourceTradeNo)
}
