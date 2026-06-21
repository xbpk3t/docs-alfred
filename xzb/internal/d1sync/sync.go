//go:generate mockgen -destination=mocks/mock_queryer.go -package=mocks github.com/xbpk3t/docs-alfred/xzb/internal/d1sync Queryer

package d1sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/d1"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
)

const insertBatchSQL = `INSERT INTO finance_import_batches (
  id, source_files, imported_at, transaction_count, dry_run
) VALUES (?, ?, ?, ?, ?)`

const upsertTransactionSQL = `INSERT INTO finance_transactions (
  id, source, source_file, source_trade_no, merchant_trade_no,
  occurred_at, month, account_type, in_out, transaction_type,
  counterparty, item_name, payment_method, status, remark,
  category, amount_cents, budget_included, budget_rule,
  import_batch_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  source_file = excluded.source_file,
  merchant_trade_no = excluded.merchant_trade_no,
  occurred_at = excluded.occurred_at,
  month = excluded.month,
  account_type = excluded.account_type,
  in_out = excluded.in_out,
  transaction_type = excluded.transaction_type,
  counterparty = excluded.counterparty,
  item_name = excluded.item_name,
  payment_method = excluded.payment_method,
  status = excluded.status,
  remark = excluded.remark,
  category = excluded.category,
  amount_cents = excluded.amount_cents,
  budget_included = excluded.budget_included,
  budget_rule = excluded.budget_rule,
  import_batch_id = excluded.import_batch_id,
  updated_at = excluded.updated_at`

type Queryer interface {
	// Query executes one D1 SQL statement with positional parameters.
	Query(ctx context.Context, sql string, params []any) (QueryResult, error)
}

type QueryResult struct {
	RowsWritten int64
}

type CloudflareQueryer struct {
	client     *cloudflare.Client
	AccountID  string
	DatabaseID string
}

type SyncSummary struct {
	BatchID     string `json:"batchId"`
	Processed   int    `json:"processed"`
	RowsWritten int64  `json:"rowsWritten"`
	SourceFiles int    `json:"sourceFiles"`
}

func NewCloudflareQueryer(accountID, apiToken, databaseID string) *CloudflareQueryer {
	client := cloudflare.NewClient(option.WithAPIToken(apiToken))

	return &CloudflareQueryer{client: client, AccountID: accountID, DatabaseID: databaseID}
}

func (q *CloudflareQueryer) Query(ctx context.Context, sql string, params []any) (QueryResult, error) {
	page, err := q.client.D1.Database.Query(ctx, q.DatabaseID, d1.DatabaseQueryParams{
		AccountID: cloudflare.F(q.AccountID),
		Body: d1.DatabaseQueryParamsBody{
			Sql:    cloudflare.F(sql),
			Params: cloudflare.F[any](params),
		},
	})
	if err != nil {
		return QueryResult{}, err
	}

	var rowsWritten int64
	for i := range page.Result {
		rowsWritten += int64(page.Result[i].Meta.RowsWritten)
	}

	return QueryResult{RowsWritten: rowsWritten}, nil
}

func Sync(ctx context.Context, q Queryer, transactions []model.Transaction, sourceFiles []string, now time.Time) (SyncSummary, error) {
	batchID := "xzb:" + now.UTC().Format("20060102T150405.000000000Z")
	encodedFiles, err := json.Marshal(sourceFiles)
	if err != nil {
		return SyncSummary{}, err
	}

	batchResult, err := q.Query(ctx, insertBatchSQL, []any{
		batchID,
		string(encodedFiles),
		formatTime(now),
		len(transactions),
		0,
	})
	if err != nil {
		return SyncSummary{}, fmt.Errorf("insert import batch: %w", err)
	}

	summary := SyncSummary{
		BatchID:     batchID,
		Processed:   len(transactions),
		RowsWritten: batchResult.RowsWritten,
		SourceFiles: len(sourceFiles),
	}

	for i := range transactions {
		transaction := &transactions[i]
		result, err := q.Query(ctx, upsertTransactionSQL, transactionParams(transaction, batchID))
		if err != nil {
			return summary, fmt.Errorf("upsert transaction %s: %w", transaction.ID, err)
		}
		summary.RowsWritten += result.RowsWritten
	}

	return summary, nil
}

func SQLScript(transactions []model.Transaction, sourceFiles []string, now time.Time) (string, SyncSummary, error) {
	batchID := "xzb:" + now.UTC().Format("20060102T150405.000000000Z")
	encodedFiles, err := json.Marshal(sourceFiles)
	if err != nil {
		return "", SyncSummary{}, err
	}

	var builder strings.Builder
	builder.WriteString("BEGIN TRANSACTION;\n")
	builder.WriteString(renderSQL(insertBatchSQL, []any{
		batchID,
		string(encodedFiles),
		formatTime(now),
		len(transactions),
		0,
	}))
	builder.WriteString(";\n")

	for i := range transactions {
		builder.WriteString(renderSQL(upsertTransactionSQL, transactionParams(&transactions[i], batchID)))
		builder.WriteString(";\n")
	}
	builder.WriteString("COMMIT;\n")

	return builder.String(), SyncSummary{
		BatchID:     batchID,
		Processed:   len(transactions),
		SourceFiles: len(sourceFiles),
	}, nil
}

func transactionParams(t *model.Transaction, batchID string) []any {
	return []any{
		t.ID,
		string(t.Source),
		t.SourceFile,
		t.SourceTradeNo,
		t.MerchantTradeNo,
		formatTime(t.OccurredAt),
		t.Month,
		t.AccountType,
		t.InOut,
		t.TransactionType,
		t.Counterparty,
		t.ItemName,
		t.PaymentMethod,
		t.Status,
		t.Remark,
		t.Category,
		t.AmountCents,
		boolInt(t.BudgetIncluded),
		t.BudgetRule,
		batchID,
		formatTime(t.CreatedAt),
		formatTime(t.UpdatedAt),
	}
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func boolInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

func renderSQL(query string, params []any) string {
	var builder strings.Builder
	paramIndex := 0
	for _, char := range query {
		if char == '?' && paramIndex < len(params) {
			builder.WriteString(sqlLiteral(params[paramIndex]))
			paramIndex++

			continue
		}
		builder.WriteRune(char)
	}

	return builder.String()
}

func sqlLiteral(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "1"
		}

		return "0"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	}
}
