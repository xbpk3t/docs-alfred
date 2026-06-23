package parser

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
	"github.com/xuri/excelize/v2"
)

var wechatRequiredColumns = []string{
	colTradeTime,
	colTransactionType,
	colCounterparty,
	colInOut,
	colAmountCNAlt,
	colCurrentStatus,
	colTradeOrderNo,
}

func ParseWechatFiles(paths []string) (ParseResult, error) {
	var result ParseResult
	for _, path := range paths {
		records, err := ParseWechatFile(path)
		if err != nil {
			return result, fmt.Errorf("parse wechat %s: %w", path, err)
		}
		appendFileResult(&result, path, model.SourceWechat, records)
	}

	return result, nil
}

func ParseWechatFile(path string) ([]model.ParsedTransaction, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".xlsx" || ext == ".xls" {
		return parseWechatXLSX(path)
	}
	rows, err := readCSVRows(path, "utf-8")
	if err != nil {
		return nil, err
	}

	return parseWechatRows(path, rows)
}

func parseWechatXLSX(path string) ([]model.ParsedTransaction, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	return parseWechatRows(path, rows)
}

func parseWechatRows(path string, rows [][]string) ([]model.ParsedTransaction, error) {
	return parseRows(path, rows, wechatRequiredColumns, parseWechatTransaction)
}

func parseWechatTransaction(
	path string,
	row []string,
	indexes map[string]int,
	rowNumber int,
) (model.ParsedTransaction, bool, error) {
	if len(row) == 0 {
		return model.ParsedTransaction{}, false, nil
	}

	inOut := get(row, indexes, colInOut)
	remark := cleanSlash(get(row, indexes, "备注"))
	if shouldSkipWechatTransaction(inOut, remark, cleanSlash(get(row, indexes, colCurrentStatus))) {
		return model.ParsedTransaction{}, false, nil
	}

	amountCents, err := AmountCents(get(row, indexes, colAmountCNAlt, colAmountCN))
	if err != nil {
		return model.ParsedTransaction{}, false, fmt.Errorf("row %d amount: %w", rowNumber, err)
	}
	if amountCents == 0 {
		return model.ParsedTransaction{}, false, nil
	}

	occurredAt, err := ParseTime(get(row, indexes, colTradeTime))
	if err != nil {
		return model.ParsedTransaction{}, false, fmt.Errorf("row %d time: %w", rowNumber, err)
	}

	return wechatTransaction(path, row, indexes, occurredAt, inOut, remark, amountCents), true, nil
}

func shouldSkipWechatTransaction(inOut, remark, status string) bool {
	return status == "已全额退款" || inOut == "/" && (remark == "" || strings.Contains(remark, "服务费"))
}

func wechatTransaction(
	path string,
	row []string,
	indexes map[string]int,
	occurredAt time.Time,
	inOut,
	remark string,
	amountCents int64,
) model.ParsedTransaction {
	return model.ParsedTransaction{
		OccurredAt:      occurredAt,
		Source:          model.SourceWechat,
		SourceFile:      sourceFile(path),
		SourceTradeNo:   cleanSlash(get(row, indexes, colTradeOrderNo)),
		MerchantTradeNo: cleanSlash(get(row, indexes, "商户单号")),
		AccountType:     "微信",
		InOut:           inOut,
		TransactionType: cleanSlash(get(row, indexes, colTransactionType)),
		Counterparty:    cleanSlash(get(row, indexes, colCounterparty)),
		ItemName:        cleanSlash(get(row, indexes, "商品", "商品名称")),
		PaymentMethod:   cleanSlash(get(row, indexes, "支付方式")),
		Status:          cleanSlash(get(row, indexes, colCurrentStatus)),
		Remark:          remark,
		AmountCents:     amountCents,
	}
}
