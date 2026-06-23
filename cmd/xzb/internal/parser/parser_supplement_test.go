package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
	"github.com/xuri/excelize/v2"
)

func TestAmountCentsEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"empty", "", 0, false},
		{"slash", "/", 0, false},
		{"zero", "0", 0, false},
		{"zero decimal", "0.00", 0, false},
		{"positive sign", "+5.00", 500, false},
		{"negative", "-3.50", -350, false},
		{"yuan only", "100", 10000, false},
		{"single decimal", "5.5", 550, false},
		{"two decimals", "5.55", 555, false},
		{"comma separated", "¥1,000.00", 100000, false},
		{"fullwidth yen", "￥100.00", 10000, false},
		{"space padded", "  ¥50.00  ", 5000, false},
		{"tab padded", "\t10.00\t", 1000, false},
		{"three decimals", "5.555", 0, true},
		{"invalid text", "abc", 0, true},
		{"multiple dots", "1.2.3", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AmountCents(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestNonNegativeNetAmount(t *testing.T) {
	tests := []struct {
		name        string
		amountCents int64
		refundCents int64
		want        int64
	}{
		{"no refund", 500, 0, 500},
		{"partial refund", 500, 200, 300},
		{"full refund", 500, 500, 0},
		{"over refund", 500, 700, 0},
		{"both zero", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, NonNegativeNetAmount(tt.amountCents, tt.refundCents))
		})
	}
}

func TestCleanCell(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain", "hello", "hello"},
		{"leading space", "  hello", "hello"},
		{"trailing space", "hello  ", "hello"},
		{"BOM", "﻾hello", "﻾hello"},
		{"LTR mark", "‎hello‏", "hello"},
		{"all combined", "  ﻾‎hello‏  ", "﻾‎hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, cleanCell(tt.input))
		})
	}
}

func TestCleanSlash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"slash only", "/", ""},
		{"leading slash", "/foo", "foo"},
		{"trailing slash", "foo/", "foo"},
		{"both slashes", "/foo/", "foo"},
		{"with spaces", " /foo/ ", "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, cleanSlash(tt.input))
		})
	}
}

func TestNormalizeHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "hello", "hello"},
		{"with spaces", "hello world", "helloworld"},
		{"with tab", "hello\tworld", "helloworld"},
		{"with newline", "hello\nworld", "helloworld"},
		{"BOM prefix", "﻾hello", "﻾hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeHeader(tt.input))
		})
	}
}

func TestGet(t *testing.T) {
	indexes := map[string]int{"name": 0, "age": 1, "city": 2}
	row := []string{"Alice", "30", "Beijing"}

	require.Equal(t, "Alice", get(row, indexes, "name"))
	require.Equal(t, "30", get(row, indexes, "age"))
	require.Equal(t, "Beijing", get(row, indexes, "city"))
	require.Equal(t, "", get(row, indexes, "missing"))
	require.Equal(t, "Alice", get(row, indexes, "missing", "name"))
	require.Equal(t, "", get(row, indexes))
}

func TestGetOutOfBounds(t *testing.T) {
	indexes := map[string]int{"col": 5}
	row := []string{"a", "b"}
	require.Equal(t, "", get(row, indexes, "col"))
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"standard", "2026-05-01 10:30:00", false},
		{"date only", "2026-05-01", false},
		{"with BOM", "﻾2026-05-01 10:30:00", true},
		{"empty", "", false},
		{"garbage", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTime(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSourceFile(t *testing.T) {
	require.Equal(t, "file.csv", sourceFile("/path/to/file.csv"))
	require.Equal(t, "file.csv", sourceFile("file.csv"))
}

func TestAppendFileResult(t *testing.T) {
	var result ParseResult
	records := []model.ParsedTransaction{
		{SourceTradeNo: "1"},
		{SourceTradeNo: "2"},
	}
	appendFileResult(&result, "/path/to/file.csv", model.SourceWechat, records)

	require.Len(t, result.Records, 2)
	require.Len(t, result.Files, 1)
	require.Equal(t, "/path/to/file.csv", result.Files[0].Path)
	require.Equal(t, model.SourceWechat, result.Files[0].Source)
	require.Equal(t, 2, result.Files[0].Records)
}

func TestNormalizeTransactionsDedup(t *testing.T) {
	now := time.Now()
	records := []model.ParsedTransaction{
		{Source: model.SourceWechat, SourceTradeNo: "trade-1", OccurredAt: now, AmountCents: 100},
		{Source: model.SourceWechat, SourceTradeNo: "trade-1", OccurredAt: now, AmountCents: 100},
		{Source: model.SourceWechat, SourceTradeNo: "trade-2", OccurredAt: now, AmountCents: 200},
	}
	transactions := NormalizeTransactions(records, now)
	require.Len(t, transactions, 2)
}

func TestStableIDWithTradeNo(t *testing.T) {
	tx := &model.Transaction{Source: model.SourceAlipay, SourceTradeNo: "ali-123"}
	require.Equal(t, "alipay:ali-123", StableID(tx))
}

func TestStableIDHashDeterministic(t *testing.T) {
	tx := &model.Transaction{
		Source:        model.SourceWechat,
		OccurredAt:    time.Date(2026, 5, 1, 8, 30, 0, 0, time.UTC),
		Counterparty:  "商户",
		ItemName:      "商品",
		InOut:         "支出",
		AmountCents:   100,
		PaymentMethod: "零钱",
		Status:        "支付成功",
		Remark:        "test",
	}
	id1 := StableID(tx)
	id2 := StableID(tx)
	require.Equal(t, id1, id2)
	require.Contains(t, id1, "wechat:hash:")
}

func TestParseWechatFileCSV(t *testing.T) {
	path := filepath.Join("testdata", "wechat_sample.csv")
	if _, err := os.Stat(path); err != nil {
		t.Skip("testdata not available")
	}
	records, err := ParseWechatFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	for _, r := range records {
		require.Equal(t, model.SourceWechat, r.Source)
	}
}

func TestParseAlipayFileCSV(t *testing.T) {
	path := filepath.Join("testdata", "alipay_sample.csv")
	if _, err := os.Stat(path); err != nil {
		t.Skip("testdata not available")
	}
	records, err := ParseAlipayFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	for _, r := range records {
		require.Equal(t, model.SourceAlipay, r.Source)
	}
}

func TestParseWechatFileNotFound(t *testing.T) {
	_, err := ParseWechatFile("/nonexistent/file.csv")
	require.Error(t, err)
}

func TestParseAlipayFileNotFound(t *testing.T) {
	_, err := ParseAlipayFile("/nonexistent/file.csv")
	require.Error(t, err)
}

func TestParseWechatFilesEmpty(t *testing.T) {
	result, err := ParseWechatFiles(nil)
	require.NoError(t, err)
	require.Empty(t, result.Records)
	require.Empty(t, result.Files)
}

func TestParseAlipayFilesEmpty(t *testing.T) {
	result, err := ParseAlipayFiles(nil)
	require.NoError(t, err)
	require.Empty(t, result.Records)
	require.Empty(t, result.Files)
}

func TestShouldSkipAlipayTransaction(t *testing.T) {
	tests := []struct {
		name   string
		inOut  string
		expect bool
	}{
		{"empty", "", true},
		{"other", "其他", true},
		{"not counted", "不计收支", true},
		{"slash", "/", true},
		{"income", "收入", false},
		{"expense", "支出", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, shouldSkipAlipayTransaction(tt.inOut))
		})
	}
}

func TestShouldSkipWechatTransaction(t *testing.T) {
	tests := []struct {
		name   string
		inOut  string
		remark string
		status string
		expect bool
	}{
		{"refunded", "支出", "", "已全额退款", true},
		{"slash no remark", "/", "", "支付成功", true},
		{"slash with fee", "/", "服务费", "支付成功", true},
		{"slash with remark", "/", "some remark", "支付成功", false},
		{"normal", "支出", "", "支付成功", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, shouldSkipWechatTransaction(tt.inOut, tt.remark, tt.status))
		})
	}
}

func TestFindHeaderRequired(t *testing.T) {
	rows := [][]string{
		{"Name", "Age", "City"},
		{"Alice", "30", "Beijing"},
	}
	h, err := findHeader(rows, []string{"Name", "Age"})
	require.NoError(t, err)
	require.Equal(t, 0, h.rowIndex)
	require.Contains(t, h.indexes, normalizeHeader("Name"))
	require.Contains(t, h.indexes, normalizeHeader("Age"))
}

func TestFindHeaderNotFound(t *testing.T) {
	rows := [][]string{
		{"Name", "Age"},
		{"Alice", "30"},
	}
	_, err := findHeader(rows, []string{"Name", "Missing"})
	require.Error(t, err)
}

func TestFindHeaderInSecondRow(t *testing.T) {
	rows := [][]string{
		{"Header info line"},
		{"Col1", "Col2", "Col3"},
		{"data1", "data2", "data3"},
	}
	h, err := findHeader(rows, []string{"Col1", "Col3"})
	require.NoError(t, err)
	require.Equal(t, 1, h.rowIndex)
}

func TestReadCSVRowsUTF8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	require.NoError(t, os.WriteFile(path, []byte("Name,Age\nAlice,30\n"), 0600))

	rows, err := readCSVRows(path, "utf-8")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, []string{"Name", "Age"}, rows[0])
	require.Equal(t, []string{"Alice", "30"}, rows[1])
}

func TestReadCSVRowsLazyQuotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	require.NoError(t, os.WriteFile(path, []byte("Name,Desc\nAlice,\"hello \"world\"\"\n"), 0600))

	rows, err := readCSVRows(path, "utf-8")
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestReadCSVRowsNonUTF8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_gb18030.csv")
	// Write GB18030-encoded "Name,Age\nAlice,30\n"
	// "Name" = 0x4E,0x61,0x6D,0x65 (same as ASCII), "Age" = same
	// Just use ASCII content which is valid both for UTF-8 and GB18030
	require.NoError(t, os.WriteFile(path, []byte("Name,Age\nAlice,30\n"), 0600))

	rows, err := readCSVRows(path, "gb18030")
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestReadCSVRowsFileNotFound(t *testing.T) {
	_, err := readCSVRows("/nonexistent/file.csv", "utf-8")
	require.Error(t, err)
}

func TestParseAlipayFilesError(t *testing.T) {
	_, err := ParseAlipayFiles([]string{"/nonexistent/file.csv"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse alipay")
}

func TestParseWechatFilesError(t *testing.T) {
	_, err := ParseWechatFiles([]string{"/nonexistent/file.csv"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse wechat")
}

func TestParseRowsError(t *testing.T) {
	rows := [][]string{
		{"Col1", "Col2"},
		{"data1", "data2"},
	}
	badParser := func(_ string, _ []string, _ map[string]int, _ int) (model.ParsedTransaction, bool, error) {
		return model.ParsedTransaction{}, false, fmt.Errorf("parse failed")
	}
	_, err := parseRows("test.csv", rows, []string{"Col1", "Col2"}, badParser)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse failed")
}

func TestAmountCentsWithBOM(t *testing.T) {
	got, err := AmountCents("\ufeff100.00")
	require.NoError(t, err)
	require.Equal(t, int64(10000), got)
}

func TestAmountCentsNegativeSign(t *testing.T) {
	got, err := AmountCents("-50.00")
	require.NoError(t, err)
	require.Equal(t, int64(-5000), got)
}

func TestAmountCentsEmptyFraction(t *testing.T) {
	// "100." splits to ["100", ""] → empty centsText fails ParseInt
	_, err := AmountCents("100.")
	require.Error(t, err)
}

func TestSplitAmountPartsTooManyDecimals(t *testing.T) {
	_, err := splitAmountParts("1.234", "1.234")
	require.Error(t, err)
	require.Contains(t, err.Error(), "more than two decimal places")
}

func TestParseWechatFileXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wechat.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"交易时间", "交易类型", "交易对方", "商品", "收/支", "金额(元)", "支付方式", "当前状态", "交易单号", "商户单号", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	row2 := []string{"2026-05-01 08:30:00", "商户消费", "麦当劳", "早餐", "支出", "¥35.50", "微信零钱", "支付成功", "wx-trade-1", "wx-merchant-1", "/"}
	for i, v := range row2 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}
	require.NoError(t, f.SaveAs(path))
	require.NoError(t, f.Close())

	records, err := ParseWechatFile(path)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, model.SourceWechat, records[0].Source)
	require.Equal(t, "wx-trade-1", records[0].SourceTradeNo)
}

func TestParseWechatXLSXError(t *testing.T) {
	_, err := parseWechatXLSX("/nonexistent/file.xlsx")
	require.Error(t, err)
}

func TestParseAlipayEmptyRow(t *testing.T) {
	rows := [][]string{
		{"交易号", "交易创建时间", "类型", "交易对方", "商品名称", "金额（元）", "收/支", "交易状态"},
		{},
	}
	records, err := parseAlipayRows("test.csv", rows)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestParseWechatEmptyRow(t *testing.T) {
	rows := [][]string{
		{"交易时间", "交易类型", "交易对方", "收/支", "金额(元)", "当前状态", "交易单号"},
		{},
	}
	records, err := parseWechatRows("test.csv", rows)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestParseWechatZeroAmount(t *testing.T) {
	rows := [][]string{
		{"交易时间", "交易类型", "交易对方", "收/支", "金额(元)", "当前状态", "交易单号"},
		{"2026-05-01 08:30:00", "商户消费", "麦当劳", "支出", "¥0.00", "支付成功", "wx-trade-1"},
	}
	records, err := parseWechatRows("test.csv", rows)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestParseAlipaySkipped(t *testing.T) {
	rows := [][]string{
		{"交易号", "交易创建时间", "类型", "交易对方", "商品名称", "金额（元）", "收/支", "交易状态"},
		{"ali-1", "2026-05-01 08:30:00", "餐饮", "面馆", "午餐", "¥48.00", "其他", "交易成功"},
	}
	records, err := parseAlipayRows("test.csv", rows)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestParseWechatSkipped(t *testing.T) {
	rows := [][]string{
		{"交易时间", "交易类型", "交易对方", "收/支", "金额(元)", "当前状态", "交易单号"},
		{"2026-05-01 08:30:00", "商户消费", "麦当劳", "/", "¥35.50", "支付成功", "wx-trade-1"},
	}
	records, err := parseWechatRows("test.csv", rows)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestFindHeaderWithBOM(t *testing.T) {
	rows := [][]string{
		{"\ufeffName", "Age"},
		{"Alice", "30"},
	}
	h, err := findHeader(rows, []string{"Name", "Age"})
	require.NoError(t, err)
	require.Equal(t, 0, h.rowIndex)
}

func TestAlipayAmountCentsRefundError(t *testing.T) {
	indexes := map[string]int{"\u91d1\u989d\uff08\u5143\uff09": 0, "\u6210\u529f\u9000\u6b3e\uff08\u5143\uff09": 1}
	row := []string{"100.00", "invalid-refund"}
	_, err := alipayAmountCents(row, indexes, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "row 1 refund")
}

func TestParseAlipayTransactionTimeError(t *testing.T) {
	rows := [][]string{
		{"\u4ea4\u6613\u53f7", "\u4ea4\u6613\u521b\u5efa\u65f6\u95f4", "\u7c7b\u578b", "\u4ea4\u6613\u5bf9\u65b9", "\u5546\u54c1\u540d\u79f0", "\u91d1\u989d\uff08\u5143\uff09", "\u6536/\u652f", "\u4ea4\u6613\u72b6\u6001"},
		{"ali-1", "not-a-date", "\u9910\u996e", "\u9762\u9986", "\u5348\u9910", "\u00a548.00", "\u652f\u51fa", "\u4ea4\u6613\u6210\u529f"},
	}
	records, err := parseAlipayRows("test.csv", rows)
	require.Error(t, err)
	require.Nil(t, records)
	require.Contains(t, err.Error(), "row 2 time")
}

func TestParseWechatTransactionTimeError(t *testing.T) {
	rows := [][]string{
		{"\u4ea4\u6613\u65f6\u95f4", "\u4ea4\u6613\u7c7b\u578b", "\u4ea4\u6613\u5bf9\u65b9", "\u6536/\u652f", "\u91d1\u989d(\u5143)", "\u5f53\u524d\u72b6\u6001", "\u4ea4\u6613\u5355\u53f7"},
		{"not-a-date", "\u5546\u6237\u6d88\u8d39", "\u9ea6\u5f53\u52b3", "\u652f\u51fa", "\u00a535.50", "\u652f\u4ed8\u6210\u529f", "wx-trade-1"},
	}
	records, err := parseWechatRows("test.csv", rows)
	require.Error(t, err)
	require.Nil(t, records)
	require.Contains(t, err.Error(), "row 2 time")
}

func TestSplitAmountPartsSingleDecimal(t *testing.T) {
	parts, err := splitAmountParts("5.5", "5.5")
	require.NoError(t, err)
	require.Equal(t, "5", parts.yuanText)
	require.Equal(t, "50", parts.centsText)
}

func TestSplitAmountPartsNoDecimal(t *testing.T) {
	parts, err := splitAmountParts("100", "100")
	require.NoError(t, err)
	require.Equal(t, "100", parts.yuanText)
	require.Equal(t, "00", parts.centsText)
}

func TestSplitAmountPartsEmptyYuan(t *testing.T) {
	parts, err := splitAmountParts(".50", ".50")
	require.NoError(t, err)
	require.Equal(t, "0", parts.yuanText)
	require.Equal(t, "50", parts.centsText)
}

func TestReadCSVRowsGB18030(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	// Write content with high bytes that are invalid UTF-8 but valid GB18030
	// GB18030: 0xB0 0xA1 = "\u554a"
	data := []byte{0x4E, 0x61, 0x6D, 0x65, 0x2C, 0x41, 0x67, 0x65, 0x0A, 0xB0, 0xA1, 0x2C, 0x33, 0x30, 0x0A}
	require.NoError(t, os.WriteFile(path, data, 0600))

	rows, err := readCSVRows(path, "gb18030")
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestAlipayAmountCentsAmountError(t *testing.T) {
	indexes := map[string]int{"\u91d1\u989d\uff08\u5143\uff09": 0, "\u6210\u529f\u9000\u6b3e\uff08\u5143\uff09": 1}
	row := []string{"invalid-amount", "0.00"}
	_, err := alipayAmountCents(row, indexes, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "row 1 amount")
}
