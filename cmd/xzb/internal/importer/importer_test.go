package importer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/rules"
)

func TestRunEmpty(t *testing.T) {
	result, err := Run(&Input{Now: time.Now()})
	require.NoError(t, err)
	require.Empty(t, result.Transactions)
	require.Empty(t, result.SourceFiles)
	require.Empty(t, result.Files)
}

func TestRunWechatOnly(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-01 10:00:00,商户消费,麦当劳,汉堡,零钱,支出,¥35.00,支付成功,wx-trade-1,,午餐
`), 0600))

	boolTrue := true
	cfg := &rules.Config{
		Version:    1,
		Defaults:   rules.Defaults{Category: "其他", BudgetIncluded: &boolTrue},
		Categories: []rules.Category{{Name: "餐饮", Match: rules.Match{Any: []rules.Condition{{Field: "counterparty", Contains: "麦当劳"}}}}},
	}

	result, err := Run(&Input{
		Now:         time.Now(),
		Rules:       cfg,
		WechatFiles: []string{csvPath},
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 1)
	require.Equal(t, "餐饮", result.Transactions[0].Category)
	require.Len(t, result.SourceFiles, 1)
}

func TestRunLimit(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-01 10:00:00,商户消费,A,商品1,零钱,支出,¥10.00,支付成功,wx-1,,
2026-05-02 10:00:00,商户消费,B,商品2,零钱,支出,¥20.00,支付成功,wx-2,,
2026-05-03 10:00:00,商户消费,C,商品3,零钱,支出,¥30.00,支付成功,wx-3,,
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
		Limit:       2,
	})
	require.NoError(t, err)
	require.Len(t, result.Transactions, 2)
}

func TestRunSortedByOccurredAt(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "wechat.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(`交易时间,交易类型,交易对方,商品,支付方式,收/支,金额(元),当前状态,交易单号,商户单号,备注
2026-05-03 10:00:00,商户消费,C,商品,零钱,支出,¥30.00,支付成功,wx-3,,
2026-05-01 10:00:00,商户消费,A,商品,零钱,支出,¥10.00,支付成功,wx-1,,
2026-05-02 10:00:00,商户消费,B,商品,零钱,支出,¥20.00,支付成功,wx-2,,
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
	require.Len(t, result.Transactions, 3)
	require.Equal(t, "wx-1", result.Transactions[0].SourceTradeNo)
	require.Equal(t, "wx-2", result.Transactions[1].SourceTradeNo)
	require.Equal(t, "wx-3", result.Transactions[2].SourceTradeNo)
}
