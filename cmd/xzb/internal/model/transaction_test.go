package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParsedTransactionNormalize(t *testing.T) {
	now := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	occurred := time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC)

	pt := ParsedTransaction{
		OccurredAt:      occurred,
		Source:          SourceWechat,
		SourceFile:      "wechat.csv",
		SourceTradeNo:   "wx-123",
		MerchantTradeNo: "merch-456",
		AccountType:     "微信",
		InOut:           "支出",
		TransactionType: "商户消费",
		Counterparty:    "麦当劳",
		ItemName:        "汉堡",
		PaymentMethod:   "零钱",
		Status:          "支付成功",
		Remark:          "午餐",
		AmountCents:     3500,
	}

	tx := pt.Normalize(now)

	require.Equal(t, occurred, tx.OccurredAt)
	require.Equal(t, now, tx.CreatedAt)
	require.Equal(t, now, tx.UpdatedAt)
	require.Equal(t, SourceWechat, tx.Source)
	require.Equal(t, "wechat.csv", tx.SourceFile)
	require.Equal(t, "wx-123", tx.SourceTradeNo)
	require.Equal(t, "merch-456", tx.MerchantTradeNo)
	require.Equal(t, "2026-05", tx.Month)
	require.Equal(t, "微信", tx.AccountType)
	require.Equal(t, "支出", tx.InOut)
	require.Equal(t, "商户消费", tx.TransactionType)
	require.Equal(t, "麦当劳", tx.Counterparty)
	require.Equal(t, "汉堡", tx.ItemName)
	require.Equal(t, "零钱", tx.PaymentMethod)
	require.Equal(t, "支付成功", tx.Status)
	require.Equal(t, "午餐", tx.Remark)
	require.Equal(t, int64(3500), tx.AmountCents)
	require.Empty(t, tx.ID)
	require.Empty(t, tx.Category)
	require.Empty(t, tx.BudgetRule)
	require.False(t, tx.BudgetIncluded)
}

func TestParsedTransactionNormalizeMonthFormat(t *testing.T) {
	tests := []struct {
		name     string
		occurred time.Time
		want     string
	}{
		{
			name:     "january",
			occurred: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			want:     "2026-01",
		},
		{
			name:     "december",
			occurred: time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC),
			want:     "2025-12",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := ParsedTransaction{OccurredAt: tt.occurred}
			tx := pt.Normalize(time.Now())
			require.Equal(t, tt.want, tx.Month)
		})
	}
}

func TestSourceConstants(t *testing.T) {
	require.Equal(t, SourceWechat, Source("wechat"))
	require.Equal(t, SourceAlipay, Source("alipay"))
}
