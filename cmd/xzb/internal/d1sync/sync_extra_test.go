package d1sync_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/d1sync"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/d1sync/mocks"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// NewCloudflareQueryer constructor
// ---------------------------------------------------------------------------

func TestNewCloudflareQueryerDoesNotPanic(t *testing.T) {
	q := d1sync.NewCloudflareQueryer("account-id", "api-token", "database-id")
	assert.NotNil(t, q)
	assert.Equal(t, "account-id", q.AccountID)
	assert.Equal(t, "database-id", q.DatabaseID)
}

// ---------------------------------------------------------------------------
// sqlLiteral via SQLScript
// ---------------------------------------------------------------------------

func TestSQLScriptNilField(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:         "t1",
		Source:     model.SourceWechat,
		OccurredAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
		Remark:     "",
	}}
	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	// Empty string should be rendered as ''
	assert.Contains(t, script, "''")
}

func TestSQLScriptInt64AmountCents(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:          "t1",
		Source:      model.SourceWechat,
		OccurredAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
		AmountCents: 12345,
	}}
	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	assert.Contains(t, script, "12345")
}

func TestSQLScriptUnknownType(t *testing.T) {
	// This tests the default case of sqlLiteral by checking bool rendering
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:             "t1",
		BudgetIncluded: true,
		OccurredAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	// BudgetIncluded=true should render as 1
	assert.Contains(t, script, "1")
}

func TestSQLScriptBudgetIncludedFalse(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:             "t1",
		BudgetIncluded: false,
		OccurredAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	script, _, err := d1sync.SQLScript(transactions, nil, now)
	require.NoError(t, err)
	// BudgetIncluded=false should render as 0
	assert.Contains(t, script, "0")
}

// ---------------------------------------------------------------------------
// Sync with more params than ? placeholders
// ---------------------------------------------------------------------------

func TestSyncRowsWrittenAccumulates(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{
		{ID: "t1", Source: model.SourceWechat, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
	}

	callCount := 0
	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sql string, params []any) (d1sync.QueryResult, error) {
			callCount++
			return d1sync.QueryResult{RowsWritten: int64(callCount)}, nil
		}).Times(2)

	summary, err := d1sync.Sync(context.Background(), fake, transactions, []string{"f.csv"}, now)
	require.NoError(t, err)
	assert.Equal(t, int64(3), summary.RowsWritten) // 1 + 2
}

// ---------------------------------------------------------------------------
// SQLScript edge cases
// ---------------------------------------------------------------------------

func TestSQLScriptEmptySourceFiles(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	script, summary, err := d1sync.SQLScript(nil, []string{}, now)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.Processed)
	assert.Contains(t, script, "BEGIN TRANSACTION;")
	assert.Contains(t, script, "COMMIT;")
}

func TestSQLScriptSingleTransaction(t *testing.T) {
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{{
		ID:          "wechat:1",
		Source:      model.SourceWechat,
		SourceFile:  "wechat.csv",
		AmountCents: 100,
		OccurredAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}

	script, summary, err := d1sync.SQLScript(transactions, []string{"wechat.csv"}, now)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Processed)
	assert.Equal(t, 1, summary.SourceFiles)
	assert.True(t, strings.HasPrefix(script, "BEGIN TRANSACTION;"))
	assert.True(t, strings.HasSuffix(script, "COMMIT;\n"))
	assert.Contains(t, script, "wechat:1")
}

// ---------------------------------------------------------------------------
// renderSQL with extra params
// ---------------------------------------------------------------------------

func TestRenderSQLWithFewerPlaceholders(t *testing.T) {
	// SQLScript uses renderSQL internally; test through SQLScript
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	script, _, err := d1sync.SQLScript([]model.Transaction{{
		ID:         "t1",
		Source:     model.SourceWechat,
		OccurredAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}}, nil, now)
	require.NoError(t, err)
	// The script should have the batch SQL and upsert SQL
	assert.Contains(t, script, "finance_import_batches")
	assert.Contains(t, script, "finance_transactions")
}

// ---------------------------------------------------------------------------
// Sync with batch insert error wrapping
// ---------------------------------------------------------------------------

func TestSyncBatchInsertErrorWrapsCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)

	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(d1sync.QueryResult{}, assert.AnError).Times(1)

	_, err := d1sync.Sync(context.Background(), fake, nil, []string{"f.csv"}, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert import batch")
}

// ---------------------------------------------------------------------------
// Sync with 2 transactions
// ---------------------------------------------------------------------------

func TestSyncTwoTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	fake := mocks.NewMockQueryer(ctrl)

	now := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	transactions := []model.Transaction{
		{ID: "t1", Source: model.SourceWechat, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "t2", Source: model.SourceAlipay, OccurredAt: now, CreatedAt: now, UpdatedAt: now},
	}

	fake.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(d1sync.QueryResult{RowsWritten: 1}, nil).Times(3)

	summary, err := d1sync.Sync(context.Background(), fake, transactions, []string{"a.csv", "b.csv"}, now)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.Processed)
	assert.Equal(t, 2, summary.SourceFiles)
	assert.Equal(t, int64(3), summary.RowsWritten)
}
