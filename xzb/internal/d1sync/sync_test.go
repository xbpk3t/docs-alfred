package d1sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
)

type fakeQueryer struct {
	queries []string
	params  [][]any
}

func (f *fakeQueryer) Query(_ context.Context, sql string, params []any) (QueryResult, error) {
	f.queries = append(f.queries, sql)
	f.params = append(f.params, params)
	return QueryResult{RowsWritten: 1}, nil
}

func TestSyncUsesImportBatchAndUpsert(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	fake := &fakeQueryer{}
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

	summary, err := Sync(context.Background(), fake, transactions, []string{"wechat.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Processed)
	require.Len(t, fake.queries, 2)
	require.Contains(t, fake.queries[0], "finance_import_batches")
	require.Contains(t, fake.queries[1], "ON CONFLICT(id)")
	require.Equal(t, "wechat:1", fake.params[1][0])
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

	script, summary, err := SQLScript(transactions, []string{"wechat.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Processed)
	require.Contains(t, script, "BEGIN TRANSACTION;")
	require.Contains(t, script, "finance_import_batches")
	require.Contains(t, script, "ON CONFLICT(id) DO UPDATE")
	require.Contains(t, script, "'Bob''s Store'")
	require.Contains(t, script, "COMMIT;")
}
