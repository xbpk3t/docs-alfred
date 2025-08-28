package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xbpk3t/docs-alfred/xzb/pkg/model"
)

// DBBillRecord represents a bill record in the database
type DBBillRecord struct {
	ID            string  // Unique identifier based on record content hash
	Date          string  // Transaction time
	AccountType   string  // Account type (WeChat/Alipay)
	Type          string  // Transaction type
	Counterparty  string  // Counterparty
	ItemName      string  // Item name
	InOut         string  // Income/Expense
	PaymentMethod string  // Payment method
	Status        string  // Transaction status
	TradeNo       string  // Transaction number
	MerchantNo    string  // Merchant number
	Remark        string  // Remarks
	Category      string  // Category
	CreatedAt     string  // Creation time
	Amount        float64 // Amount
}

// Summary represents summary information
type Summary struct {
	CategoryStats map[string]float64 `json:"category_stats"`
	AccountStats  map[string]float64 `json:"account_stats"`
	LastUpdatedAt string             `json:"last_updated_at"`
	TotalRecords  int                `json:"total_records"`
	TotalIncome   float64            `json:"total_income"`
	TotalExpense  float64            `json:"total_expense"`
	NetIncome     float64            `json:"net_income"`
}

// InitDB initializes database connection and creates tables
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create bills table
	createTableSQL := `CREATE TABLE IF NOT EXISTS bills (
		id TEXT PRIMARY KEY,
		date TEXT NOT NULL,
		account_type TEXT NOT NULL,
		type TEXT NOT NULL,
		counterparty TEXT NOT NULL,
		item_name TEXT NOT NULL,
		in_out TEXT NOT NULL,
		payment_method TEXT NOT NULL,
		status TEXT NOT NULL,
		trade_no TEXT NOT NULL,
		merchant_no TEXT NOT NULL,
		remark TEXT NOT NULL,
		category TEXT NOT NULL,
		amount REAL NOT NULL,
		created_at TEXT NOT NULL
	);`

	ctx := context.Background()
	_, err = db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return nil, err
	}

	// Create index to improve query performance
	createIndexSQL := `CREATE INDEX IF NOT EXISTS idx_bills_date ON bills (date);`
	_, err = db.ExecContext(ctx, createIndexSQL)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// generateRecordID generates a unique ID based on record content
func generateRecordID(record model.BillRecord) string {
	// Concatenate key fields into a string
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%f",
		record.Date,
		record.AccountType,
		record.Counterparty,
		record.ItemName,
		record.InOut,
		record.TradeNo,
		record.Amount)

	// Generate SHA256 hash as unique ID
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// InsertRecords inserts bill records into the database
func InsertRecords(dbPath string, records []model.BillRecord) error {
	db, err := InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database: %v", closeErr)
		}
	}()

	// Begin transaction to improve performance
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Insert records using transaction
	for _, record := range records {
		// Generate unique ID for the record
		recordID := generateRecordID(record)

		// Check if record already exists
		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM bills WHERE id = ?", recordID).Scan(&count)
		if err != nil {
			return err
		}

		// Skip if record already exists
		if count > 0 {
			continue
		}

		// Insert new record
		insertSQL := `INSERT INTO bills (
			id, date, account_type, type, counterparty, item_name, in_out,
			payment_method, status, trade_no, merchant_no, remark, category, amount, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

		_, err = tx.ExecContext(ctx, insertSQL,
			recordID,
			record.Date,
			record.AccountType,
			record.Type,
			record.Counterparty,
			record.ItemName,
			record.InOut,
			record.PaymentMethod,
			record.Status,
			record.TradeNo,
			record.MerchantNo,
			record.Remark,
			record.Category,
			record.Amount,
			time.Now().Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			return fmt.Errorf("failed to insert record: %w", err)
		}
	}

	// Commit transaction
	return tx.Commit()
}

// QueryRecords queries bill records from the database
func QueryRecords(dbPath string, conditions map[string]interface{}) ([]DBBillRecord, error) {
	db, err := InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database: %v", closeErr)
		}
	}()

	// Build query SQL
	query := `SELECT id, date, account_type, type, counterparty, item_name, in_out,
		payment_method, status, trade_no, merchant_no, remark, category, amount, created_at
		FROM bills`
	var args []interface{}

	// Add query conditions
	whereConditions, args := buildWhereConditions(conditions)
	if len(whereConditions) > 0 {
		query += " WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Add sorting
	query += " ORDER BY date DESC;"

	// Execute query
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Error closing rows: %v", closeErr)
		}
	}()

	// Process query results
	var records []DBBillRecord
	for rows.Next() {
		var record DBBillRecord
		err := rows.Scan(
			&record.ID,
			&record.Date,
			&record.AccountType,
			&record.Type,
			&record.Counterparty,
			&record.ItemName,
			&record.InOut,
			&record.PaymentMethod,
			&record.Status,
			&record.TradeNo,
			&record.MerchantNo,
			&record.Remark,
			&record.Category,
			&record.Amount,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetAllRecords gets all bill records
func GetAllRecords(dbPath string) ([]DBBillRecord, error) {
	return QueryRecords(dbPath, nil)
}

// CalculateSummary calculates bill summary information
func CalculateSummary(records []DBBillRecord) Summary {
	var totalIncome, totalExpense float64
	categoryStats := make(map[string]float64)
	accountStats := make(map[string]float64)

	for _, record := range records {
		switch record.InOut {
		case "收入":
			totalIncome += record.Amount
			categoryStats[record.Category] += record.Amount
			accountStats[record.AccountType] += record.Amount
		case "支出":
			totalExpense += record.Amount
			categoryStats[record.Category] -= record.Amount
			accountStats[record.AccountType] -= record.Amount
		}
	}

	return Summary{
		TotalRecords:  len(records),
		TotalIncome:   totalIncome,
		TotalExpense:  totalExpense,
		NetIncome:     totalIncome - totalExpense,
		CategoryStats: categoryStats,
		AccountStats:  accountStats,
		LastUpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
}

// EncodeJSON encodes data as JSON and writes to io.Writer
func EncodeJSON(w io.Writer, data interface{}) error {
	return json.NewEncoder(w).Encode(data)
}

// buildWhereConditions builds WHERE conditions from the given conditions map
func buildWhereConditions(conditions map[string]interface{}) ([]string, []interface{}) {
	if len(conditions) == 0 {
		return nil, nil
	}

	var whereConditions []string
	var args []interface{}

	// Query by date range
	if startDate, ok := conditions["start_date"]; ok {
		whereConditions = append(whereConditions, "date >= ?")
		args = append(args, startDate)
	}

	if endDate, ok := conditions["end_date"]; ok {
		whereConditions = append(whereConditions, "date <= ?")
		args = append(args, endDate)
	}

	// Query by category
	if category, ok := conditions["category"]; ok {
		whereConditions = append(whereConditions, "category = ?")
		args = append(args, category)
	}

	// Query by income/expense type
	if inOut, ok := conditions["in_out"]; ok {
		whereConditions = append(whereConditions, "in_out = ?")
		args = append(args, inOut)
	}

	// Query by account type
	if accountType, ok := conditions["account_type"]; ok {
		whereConditions = append(whereConditions, "account_type = ?")
		args = append(args, accountType)
	}

	// Query by counterparty
	if counterparty, ok := conditions["counterparty"]; ok {
		whereConditions = append(whereConditions, "counterparty LIKE ?")
		args = append(args, "%"+counterparty.(string)+"%")
	}

	return whereConditions, args
}
