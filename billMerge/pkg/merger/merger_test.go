package merger

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessWechatBill(t *testing.T) {
	// 测试微信账单处理
	wechatFile := filepath.Join("testdata", "wechat_test.csv")

	records, err := processWechatBill(wechatFile)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 4)

	// 转换为 model.BillRecord 用于测试
	modelRecords := convertToModelRecords(records)

	// 验证至少有一条记录符合预期
	found := false
	for _, record := range modelRecords {
		if record.Date == "2024-12-01 10:30:00" &&
			record.Counterparty == "张三" &&
			record.InOut == "收入" &&
			record.Amount == 100.0 {
			found = true
			break
		}
	}
	assert.True(t, found, "未找到预期的微信账单记录")
}

func TestProcessAlipayBill(t *testing.T) {
	// 测试支付宝账单处理
	alipayFile := filepath.Join("testdata", "alipay_test.csv")

	records, err := processAlipayBill(alipayFile)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 3)

	// 转换为 model.BillRecord 用于测试
	modelRecords := convertToModelRecords(records)

	// 验证至少有一条记录符合预期
	found := false
	for _, record := range modelRecords {
		if record.Date == "2024-12-01 10:00:00" &&
			record.Counterparty == "工资" &&
			record.InOut == "收入" &&
			record.Amount == 1000.0 {
			found = true
			break
		}
	}
	assert.True(t, found, "未找到预期的支付宝账单记录")
}

func TestDeduplicateRecords(t *testing.T) {
	// 测试去重功能
	records := []BillRecord{
		{
			Date:         "2024-12-01 10:00:00",
			Counterparty: "张三",
			ItemName:     "转账",
			InOut:        "收入",
			Amount:       100.0,
		},
		{
			Date:         "2024-12-01 10:00:00",
			Counterparty: "张三",
			ItemName:     "转账",
			InOut:        "收入",
			Amount:       100.0,
		},
		{
			Date:         "2024-12-01 11:00:00",
			Counterparty: "李四",
			ItemName:     "转账",
			InOut:        "收入",
			Amount:       200.0,
		},
	}

	// 转换为 model.BillRecord
	modelRecords := convertToModelRecords(records)

	deduped := deduplicateRecords(modelRecords)
	assert.Len(t, deduped, 2)
}

func TestGenerateMonthlySummary(t *testing.T) {
	// 测试月度汇总功能
	records := []BillRecord{
		{
			Date:   "2024-12-01 10:00:00",
			InOut:  "收入",
			Amount: 1000.0,
		},
		{
			Date:   "2024-12-01 11:00:00",
			InOut:  "支出",
			Amount: 100.0,
		},
		{
			Date:   "2024-12-02 10:00:00",
			InOut:  "支出",
			Amount: 200.0,
		},
		{
			Date:   "2025-01-01 10:00:00",
			InOut:  "收入",
			Amount: 2000.0,
		},
	}

	// 转换为 model.BillRecord
	modelRecords := convertToModelRecords(records)

	summary := generateMonthlySummary(modelRecords)
	assert.Len(t, summary, 2)

	// 验证2024-12的数据
	assert.Equal(t, "2024-12", summary[0].Month)
	assert.Equal(t, 1000.0, summary[0].Income)
	assert.Equal(t, 300.0, summary[0].Expense)
	assert.Equal(t, 3, summary[0].Records)

	// 验证2025-01的数据
	assert.Equal(t, "2025-01", summary[1].Month)
	assert.Equal(t, 2000.0, summary[1].Income)
	assert.Equal(t, 0.0, summary[1].Expense)
	assert.Equal(t, 1, summary[1].Records)
}

func TestSaveAsCSV(t *testing.T) {
	// 测试保存为CSV功能
	records := []BillRecord{
		{
			Date:        "2024-12-01 10:00:00",
			AccountType: "微信",
			InOut:       "收入",
			Amount:      100.0,
			Category:    "转账",
		},
	}

	// 转换为 model.BillRecord
	modelRecords := convertToModelRecords(records)

	// 创建临时测试文件
	tmpFile := filepath.Join(t.TempDir(), "test.csv")
	err := saveAsCSV(tmpFile, modelRecords)
	assert.NoError(t, err)

	// 验证文件内容
	file, err := os.Open(tmpFile)
	assert.NoError(t, err)
	defer file.Close()

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, lines, 2) // 标题行 + 数据行

	// 验证标题行
	assert.Equal(t, []string{"时间", "账户1", "类型", "支付状态", "交易类型", "交易对方", "备注", "金额", "分类"}, lines[0])

	// 验证数据行
	assert.Equal(t, []string{"2024-12-01 10:00:00", "微信", "收入", "", "", "", "", "100.00", "转账"}, lines[1])
}
