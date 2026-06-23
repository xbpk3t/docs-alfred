package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

var alipayRequiredColumns = []string{
	colTradeNo,
	colTradeCreateTime,
	colTradeType,
	colCounterparty,
	colItemName,
	colAmountCN,
	colInOut,
	colTradeStatus,
}

func ParseAlipayFiles(paths []string) (ParseResult, error) {
	var result ParseResult
	for _, path := range paths {
		records, err := ParseAlipayFile(path)
		if err != nil {
			return result, fmt.Errorf("parse alipay %s: %w", path, err)
		}
		appendFileResult(&result, path, model.SourceAlipay, records)
	}

	return result, nil
}

func ParseAlipayFile(path string) ([]model.ParsedTransaction, error) {
	rows, err := readCSVRows(path, "gb18030")
	if err != nil {
		return nil, err
	}

	return parseAlipayRows(path, rows)
}

func parseAlipayRows(path string, rows [][]string) ([]model.ParsedTransaction, error) {
	return parseRows(path, rows, alipayRequiredColumns, parseAlipayTransaction)
}

func parseAlipayTransaction(
	path string,
	row []string,
	indexes map[string]int,
	rowNumber int,
) (model.ParsedTransaction, bool, error) {
	if len(row) == 0 {
		return model.ParsedTransaction{}, false, nil
	}

	inOut := get(row, indexes, colInOut)
	if shouldSkipAlipayTransaction(inOut) {
		return model.ParsedTransaction{}, false, nil
	}

	amountCents, err := alipayAmountCents(row, indexes, rowNumber)
	if err != nil || amountCents == 0 {
		return model.ParsedTransaction{}, false, err
	}

	occurredAt, err := ParseTime(get(row, indexes, colTradeCreateTime, "交易付款时间", "付款时间"))
	if err != nil {
		return model.ParsedTransaction{}, false, fmt.Errorf("row %d time: %w", rowNumber, err)
	}

	return alipayTransaction(path, row, indexes, occurredAt, inOut, amountCents), true, nil
}

func shouldSkipAlipayTransaction(inOut string) bool {
	return inOut == "" || inOut == "其他" || inOut == "不计收支" || inOut == "/"
}

func alipayAmountCents(row []string, indexes map[string]int, rowNumber int) (int64, error) {
	amountCents, err := AmountCents(get(row, indexes, colAmountCN, colAmountCNAlt))
	if err != nil {
		return 0, fmt.Errorf("row %d amount: %w", rowNumber, err)
	}
	refundCents, err := AmountCents(get(row, indexes, "成功退款（元）", "成功退款(元)"))
	if err != nil {
		return 0, fmt.Errorf("row %d refund: %w", rowNumber, err)
	}

	return NonNegativeNetAmount(amountCents, refundCents), nil
}

func alipayTransaction(
	path string,
	row []string,
	indexes map[string]int,
	occurredAt time.Time,
	inOut string,
	amountCents int64,
) model.ParsedTransaction {
	return model.ParsedTransaction{
		OccurredAt:      occurredAt,
		Source:          model.SourceAlipay,
		SourceFile:      sourceFile(path),
		SourceTradeNo:   cleanSlash(get(row, indexes, colTradeNo)),
		MerchantTradeNo: cleanSlash(get(row, indexes, "商家订单号", "商户单号")),
		AccountType:     "支付宝",
		InOut:           inOut,
		TransactionType: cleanSlash(get(row, indexes, colTradeType, colTransactionType)),
		Counterparty:    cleanSlash(get(row, indexes, colCounterparty)),
		ItemName:        cleanSlash(get(row, indexes, colItemName, "商品")),
		PaymentMethod:   cleanSlash(get(row, indexes, "交易来源地")),
		Status:          strings.TrimSpace(cleanSlash(get(row, indexes, colTradeStatus))),
		Remark:          cleanSlash(get(row, indexes, "备注")),
		AmountCents:     amountCents,
	}
}
