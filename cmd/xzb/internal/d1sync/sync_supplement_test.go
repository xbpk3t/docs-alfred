package d1sync_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/d1sync"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/d1sync/mocks"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
	"go.uber.org/mock/gomock"
)

func TestSyncEmptyTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)

	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(d1sync.QueryResult{RowsWritten: 1}, nil).Times(1)

	summary, err := d1sync.Sync(context.Background(), fake, nil, []string{"file.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 0, summary.Processed)
	require.Equal(t, 1, int(summary.RowsWritten))
	require.Equal(t, 1, summary.SourceFiles)
}

func TestSyncBatchInsertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{ID: "t1"}}

	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(d1sync.QueryResult{}, context.DeadlineExceeded).Times(1)

	_, err := d1sync.Sync(context.Background(), fake, transactions, []string{"file.csv"}, now)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insert import batch")
}

func TestSyncUpsertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{ID: "t1", Source: model.SourceWechat}}

	callCount := 0
	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sql string, params []any) (d1sync.QueryResult, error) {
			callCount++
			if callCount == 1 {
				return d1sync.QueryResult{RowsWritten: 1}, nil
			}

			return d1sync.QueryResult{}, context.DeadlineExceeded
		}).Times(2)

	_, err := d1sync.Sync(context.Background(), fake, transactions, []string{"file.csv"}, now)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upsert transaction t1")
}

func TestSQLScriptMultipleTransactions(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{
		{
			ID: "t1", Source: model.SourceWechat, SourceFile: "a.csv", SourceTradeNo: "1",
			OccurredAt: now, Month: "2026-05", InOut: "支出",
			Counterparty: "Alice", Category: "餐饮", AmountCents: 100,
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "t2", Source: model.SourceAlipay, SourceFile: "b.csv", SourceTradeNo: "2",
			OccurredAt: now, Month: "2026-05", InOut: "收入",
			Counterparty: "Bob", Category: "工资", AmountCents: 50000,
			CreatedAt: now, UpdatedAt: now,
		},
	}

	script, summary, err := d1sync.SQLScript(transactions, []string{"a.csv", "b.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 2, summary.Processed)
	require.Equal(t, 2, summary.SourceFiles)
	require.True(t, strings.HasPrefix(script, "BEGIN TRANSACTION;"))
	require.True(t, strings.HasSuffix(script, "COMMIT;\n"))
	require.Contains(t, script, "t1")
	require.Contains(t, script, "t2")
	require.Contains(t, script, "Alice")
	require.Contains(t, script, "Bob")
}

func TestSQLScriptEscapesQuotes(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID: "t1", Source: model.SourceWechat, SourceFile: "a.csv",
		OccurredAt: now, Month: "2026-05", Counterparty: "It's a test",
		CreatedAt: now, UpdatedAt: now,
	}}

	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	require.Contains(t, script, "'It''s a test'")
}

func TestSQLScriptBooleanRendering(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{
		{ID: "t1", BudgetIncluded: true, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "t2", BudgetIncluded: false, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
	}

	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	// BudgetIncluded is rendered as 1/0
	require.Contains(t, script, "1")
	require.Contains(t, script, "0")
}

func TestSyncMultipleTransactionsRowsWritten(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{
		{ID: "t1", Source: model.SourceWechat, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "t2", Source: model.SourceAlipay, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "t3", Source: model.SourceWechat, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
	}

	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(d1sync.QueryResult{RowsWritten: 1}, nil).Times(4)

	summary, err := d1sync.Sync(context.Background(), fake, transactions, []string{"f.csv"}, now)
	require.NoError(t, err)
	require.Equal(t, 3, summary.Processed)
	require.Equal(t, int64(4), summary.RowsWritten)
}

func TestSQLScriptNilSourceFiles(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	script, summary, err := d1sync.SQLScript(nil, nil, now)
	require.NoError(t, err)
	require.Equal(t, 0, summary.Processed)
	require.Contains(t, script, "BEGIN TRANSACTION;")
	require.Contains(t, script, "COMMIT;")
}
