package d1sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/xzb/internal/d1sync"
	"github.com/xbpk3t/docs-alfred/xzb/internal/d1sync/mocks"
	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
	"go.uber.org/mock/gomock"
)

func TestSyncUsesImportBatchAndUpsert(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:              "wechat:1",
		Source:          model.SourceWechat,
		SourceFile:      "wechat.csv",
		SourceTradeNo:   "1",
		OccurredAt:      now,
		Month:           "2026-05",
		AccountType:     "微信",
		InOut:           "支出",
		TransactionType: "商户消费",
		Category:        "餐饮",
		AmountCents:     3500,
		BudgetIncluded:  true,
		BudgetRule:      "include-normal-expense",
		CreatedAt:       now,
		UpdatedAt:       now,
	}}

	var capturedSQLs []string
	var capturedParams [][]any
	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sql string, params []any) (d1sync.QueryResult, error) {
			capturedSQLs = append(capturedSQLs, sql)
			capturedParams = append(capturedParams, params)
			return d1sync.QueryResult{RowsWritten: 1}, nil
		}).Times(2)

	summary, err := d1sync.Sync(context.Background(), fake, transactions, []string{"wechat.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Processed)
	require.Contains(t, capturedSQLs[0], "finance_import_batches")
	require.Contains(t, capturedSQLs[1], "ON CONFLICT(id)")
	require.Equal(t, "wechat:1", capturedParams[1][0])
}

func TestSQLScriptEscapesValuesAndUsesUpsert(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:             "wechat:1",
		Source:         model.SourceWechat,
		SourceFile:     "wechat.csv",
		SourceTradeNo:  "1",
		OccurredAt:     now,
		Month:          "2026-05",
		InOut:          "支出",
		Counterparty:   "Bob's Store",
		ItemName:       "Coffee",
		Category:       "餐饮",
		AmountCents:    3500,
		BudgetIncluded: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}

	script, summary, err := d1sync.SQLScript(transactions, []string{"wechat.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Processed)
	require.Contains(t, script, "BEGIN TRANSACTION;")
	require.Contains(t, script, "finance_import_batches")
	require.Contains(t, script, "ON CONFLICT(id) DO UPDATE")
	require.Contains(t, script, "'Bob''s Store'")
	require.Contains(t, script, "COMMIT;")
}
