package parser

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
)

func TestAmountCents(t *testing.T) {
	tests := map[string]int64{
		"¥1,234.56": 123456,
		"1,234.56":  123456,
		"-12.30":    -1230,
		"12":        1200,
		"12.3":      1230,
	}
	for input, want := range tests {
		got, err := AmountCents(input)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestParseWechatFile(t *testing.T) {
	records, err := ParseWechatFile(filepath.Join("testdata", "wechat_sample.csv"))
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, model.SourceWechat, records[0].Source)
	require.Equal(t, "wechat_sample.csv", records[0].SourceFile)
	require.Equal(t, "wx-trade-1", records[0].SourceTradeNo)
	require.Equal(t, int64(3550), records[0].AmountCents)
	require.Equal(t, "2026-05", records[0].OccurredAt.Format("2006-01"))
}

func TestParseAlipayFile(t *testing.T) {
	records, err := ParseAlipayFile(filepath.Join("testdata", "alipay_sample.csv"))
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, model.SourceAlipay, records[0].Source)
	require.Equal(t, "alipay_sample.csv", records[0].SourceFile)
	require.Equal(t, "ali-trade-1", records[0].SourceTradeNo)
	require.Equal(t, int64(4000), records[0].AmountCents)
}

func TestStableID(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 30, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	withTradeNo := model.Transaction{Source: model.SourceWechat, SourceTradeNo: "trade-1"}
	require.Equal(t, "wechat:trade-1", StableID(&withTradeNo))

	fallback := model.Transaction{
		Source:        model.SourceWechat,
		OccurredAt:    now,
		Counterparty:  "商户",
		ItemName:      "商品",
		InOut:         "支出",
		AmountCents:   100,
		PaymentMethod: "零钱",
		Status:        "支付成功",
		Remark:        "",
	}
	first := StableID(&fallback)
	second := StableID(&fallback)
	require.Equal(t, first, second)
	require.Contains(t, first, "wechat:hash:")
}
