package parser

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type ParseResult struct {
	Records []model.ParsedTransaction
	Files   []FileResult
}

type FileResult struct {
	Path    string
	Source  model.Source
	Records int
}

type headerMatch struct {
	indexes  map[string]int
	rowIndex int
}

type rowParser func(
	path string,
	row []string,
	indexes map[string]int,
	rowNumber int,
) (model.ParsedTransaction, bool, error)

func NormalizeTransactions(records []model.ParsedTransaction, now time.Time) []model.Transaction {
	transactions := make([]model.Transaction, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for i := range records {
		transaction := records[i].Normalize(now)
		transaction.ID = StableID(&transaction)
		if _, ok := seen[transaction.ID]; ok {
			continue
		}
		seen[transaction.ID] = struct{}{}
		transactions = append(transactions, transaction)
	}

	return transactions
}

func StableID(t *model.Transaction) string {
	if t.SourceTradeNo != "" {
		return string(t.Source) + ":" + t.SourceTradeNo
	}

	fields := []string{
		string(t.Source),
		t.OccurredAt.Format("2006-01-02 15:04:05"),
		t.Counterparty,
		t.ItemName,
		t.InOut,
		strconv.FormatInt(t.AmountCents, 10),
		t.PaymentMethod,
		t.Status,
		t.Remark,
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x1f")))

	return string(t.Source) + ":hash:" + hex.EncodeToString(sum[:])[:32]
}

func ParseTime(value string) (time.Time, error) {
	s := cleanCell(value)
	formats := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
		"2006-01-02",
		"2006/01/02",
	}
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, format := range formats {
		parsed, err := time.ParseInLocation(format, s, loc)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time %q", value)
}

func readCSVRows(path, encodingHint string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var reader io.Reader = strings.NewReader(string(data))
	if !utf8.Valid(data) && encodingHint != "utf-8" {
		reader = transform.NewReader(strings.NewReader(string(data)), simplifiedchinese.GB18030.NewDecoder())
	}

	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	return csvReader.ReadAll()
}

func findHeader(rows [][]string, required []string) (headerMatch, error) {
	for i, row := range rows {
		indexes := make(map[string]int, len(row))
		for idx, col := range row {
			indexes[normalizeHeader(col)] = idx
		}

		matched := true
		for _, name := range required {
			if _, ok := indexes[normalizeHeader(name)]; !ok {
				matched = false

				break
			}
		}
		if matched {
			return headerMatch{rowIndex: i, indexes: indexes}, nil
		}
	}

	return headerMatch{}, fmt.Errorf("header with required columns %v not found", required)
}

func parseRows(path string, rows [][]string, requiredColumns []string, parse rowParser) ([]model.ParsedTransaction, error) {
	header, err := findHeader(rows, requiredColumns)
	if err != nil {
		return nil, err
	}

	records := make([]model.ParsedTransaction, 0, len(rows)-header.rowIndex-1)
	for rowNumber, row := range rows[header.rowIndex+1:] {
		record, ok, err := parse(path, row, header.indexes, header.rowIndex+rowNumber+2)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

func get(row []string, indexes map[string]int, names ...string) string {
	for _, name := range names {
		idx, ok := indexes[normalizeHeader(name)]
		if ok && idx >= 0 && idx < len(row) {
			return cleanCell(row[idx])
		}
	}

	return ""
}

func cleanCell(value string) string {
	s := strings.TrimSpace(value)
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.Trim(s, "\u200e\u200f")

	return strings.TrimSpace(s)
}

func cleanSlash(value string) string {
	s := cleanCell(value)

	return strings.Trim(strings.Trim(s, "/"), " ")
}

func normalizeHeader(value string) string {
	s := cleanCell(value)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "")

	return replacer.Replace(s)
}

func sourceFile(path string) string {
	return filepath.Base(path)
}

func appendFileResult(result *ParseResult, path string, source model.Source, records []model.ParsedTransaction) {
	result.Records = append(result.Records, records...)
	result.Files = append(result.Files, FileResult{Path: path, Source: source, Records: len(records)})
}
