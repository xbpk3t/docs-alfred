package merger

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/billMerge/pkg/classifier"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// BillRecord 表示一条账单记录
type BillRecord struct {
	Date          string  // 交易时间
	AccountType   string  // 账户类型（微信/支付宝）
	Type          string  // 交易类型
	Counterparty  string  // 交易对方
	ItemName      string  // 商品名称
	InOut         string  // 收支
	PaymentMethod string  // 支付方式
	Status        string  // 交易状态
	TradeNo       string  // 交易单号
	MerchantNo    string  // 商户单号
	Remark        string  // 备注
	Category      string  // 分类
	Amount        float64 // 金额
}

// MonthlySummary 月度汇总
type MonthlySummary struct {
	Month   string
	Income  float64
	Expense float64
	Records int
}

// MergeBills 合并微信和支付宝账单
func MergeBills(wechatFile, alipayFile string) error {
	var records []BillRecord

	// 处理微信账单
	if wechatFile != "" {
		wechatRecords, err := processWechatBill(wechatFile)
		if err != nil {
			return fmt.Errorf("处理微信账单失败: %w", err)
		}
		records = append(records, wechatRecords...)
		fmt.Printf("成功读取微信账单 %d 条\n", len(wechatRecords))
	}

	// 处理支付宝账单
	if alipayFile != "" {
		alipayRecords, err := processAlipayBill(alipayFile)
		if err != nil {
			return fmt.Errorf("处理支付宝账单失败: %w", err)
		}
		records = append(records, alipayRecords...)
		fmt.Printf("成功读取支付宝账单 %d 条\n", len(alipayRecords))
	}

	// 应用分类
	classifier, err := classifier.NewClassifier("config/category.yaml")
	if err != nil {
		fmt.Printf("警告: 无法加载分类配置: %v，默认分类为'其它'\n", err)
	} else {
		for i := range records {
			records[i].Category = classifier.Classify(
				records[i].InOut,
				records[i].Type,
				records[i].Counterparty,
				records[i].Remark,
			)
		}
	}

	// 去重
	records = deduplicateRecords(records)
	fmt.Printf("去重后总条数: %d\n", len(records))

	// 按月份分组并保存
	err = saveMonthlyBills(records)
	if err != nil {
		return fmt.Errorf("保存账单失败: %w", err)
	}

	// 保存总表
	err = saveAllBills(records)
	if err != nil {
		return fmt.Errorf("保存总表失败: %w", err)
	}

	// 生成月度汇总报告
	summary := generateMonthlySummary(records)
	err = saveMonthlySummary(summary)
	if err != nil {
		return fmt.Errorf("保存月度汇总失败: %w", err)
	}

	return nil
}

// processWechatBill 处理微信账单
func processWechatBill(filename string) ([]BillRecord, error) {
	var records []BillRecord

	// 检查是否为xlsx文件
	if strings.HasSuffix(filename, ".xlsx") || strings.HasSuffix(filename, ".xls") {
		return processWechatXLSX(filename)
	}

	// 处理CSV文件
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// 设置更宽松的读取选项
	reader.FieldsPerRecord = -1 // 允许不同数量的字段
	reader.LazyQuotes = true    // 宽松引号处理

	// 跳过前16行元数据
	for i := 0; i < 16; i++ {
		_, err := reader.Read()
		if err != nil {
			return nil, err
		}
	}

	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(lines) < 2 {
		return records, nil
	}

	// 获取标题行
	header := lines[0]

	// 找到各列索引
	dateIdx := getIndex(header, "交易时间")
	typeIdx := getIndex(header, "交易类型")
	counterpartyIdx := getIndex(header, "交易对方")
	itemIdx := getIndex(header, "商品")
	inOutIdx := getIndex(header, "收/支")
	amountIdx := getIndex(header, "金额(元)")
	paymentMethodIdx := getIndex(header, "支付方式")
	statusIdx := getIndex(header, "当前状态")
	tradeNoIdx := getIndex(header, "交易单号")
	merchantNoIdx := getIndex(header, "商户单号")
	remarkIdx := getIndex(header, "备注")

	// 处理数据行
	for i := 1; i < len(lines); i++ {
		row := lines[i]
		if len(row) <= remarkIdx || len(row) <= dateIdx || dateIdx < 0 {
			continue
		}

		// 跳过空的收支记录
		if inOutIdx >= 0 && row[inOutIdx] == "/" && (remarkIdx >= 0 && (row[remarkIdx] == "/" || strings.Contains(row[remarkIdx], "服务费"))) {
			continue
		}

		// 跳过金额为0的记录
		if amountIdx >= 0 {
			amountStr := strings.TrimPrefix(row[amountIdx], "¥")
			if amountStr == "0.00" || amountStr == "0" {
				continue
			}
		}

		record := BillRecord{
			Date:          getDateValue(row, dateIdx),
			AccountType:   "微信",
			Type:          getStringValue(row, typeIdx),
			Counterparty:  strings.Trim(getStringValue(row, counterpartyIdx), "/ "),
			ItemName:      strings.Trim(getStringValue(row, itemIdx), "/ "),
			InOut:         getStringValue(row, inOutIdx),
			PaymentMethod: getStringValue(row, paymentMethodIdx),
			Status:        strings.Trim(getStringValue(row, statusIdx), "/ "),
			TradeNo:       strings.Trim(getStringValue(row, tradeNoIdx), "/ "),
			MerchantNo:    strings.Trim(getStringValue(row, merchantNoIdx), "/ "),
			Remark:        strings.Trim(getStringValue(row, remarkIdx), "/ "),
			Category:      "其它",
		}

		// 解析金额
		if amountIdx >= 0 {
			amountStr := strings.TrimPrefix(row[amountIdx], "¥")
			fmt.Sscanf(amountStr, "%f", &record.Amount)
		}

		// 跳过已全额退款的记录
		if record.Status == "已全额退款" {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}

// processWechatXLSX 处理微信XLSX账单
func processWechatXLSX(filename string) ([]BillRecord, error) {
	var records []BillRecord

	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	// 跳过前16行元数据，找到标题行
	if len(rows) <= 16 {
		return records, nil
	}

	header := rows[16]

	// 找到各列索引
	dateIdx := getIndex(header, "交易时间")
	typeIdx := getIndex(header, "交易类型")
	counterpartyIdx := getIndex(header, "交易对方")
	itemIdx := getIndex(header, "商品")
	inOutIdx := getIndex(header, "收/支")
	amountIdx := getIndex(header, "金额(元)")
	paymentMethodIdx := getIndex(header, "支付方式")
	statusIdx := getIndex(header, "当前状态")
	tradeNoIdx := getIndex(header, "交易单号")
	merchantNoIdx := getIndex(header, "商户单号")
	remarkIdx := getIndex(header, "备注")

	// 处理数据行
	for i := 17; i < len(rows); i++ {
		row := rows[i]
		if len(row) <= remarkIdx || len(row) <= dateIdx || dateIdx < 0 {
			continue
		}

		// 跳过空的收支记录
		if inOutIdx >= 0 && row[inOutIdx] == "/" && (remarkIdx >= 0 && (row[remarkIdx] == "/" || strings.Contains(row[remarkIdx], "服务费"))) {
			continue
		}

		// 跳过金额为0的记录
		if amountIdx >= 0 {
			amountStr := strings.TrimPrefix(row[amountIdx], "¥")
			if amountStr == "0.00" || amountStr == "0" {
				continue
			}
		}

		record := BillRecord{
			Date:          getDateValue(row, dateIdx),
			AccountType:   "微信",
			Type:          getStringValue(row, typeIdx),
			Counterparty:  strings.Trim(getStringValue(row, counterpartyIdx), "/ "),
			ItemName:      strings.Trim(getStringValue(row, itemIdx), "/ "),
			InOut:         getStringValue(row, inOutIdx),
			PaymentMethod: getStringValue(row, paymentMethodIdx),
			Status:        strings.Trim(getStringValue(row, statusIdx), "/ "),
			TradeNo:       strings.Trim(getStringValue(row, tradeNoIdx), "/ "),
			MerchantNo:    strings.Trim(getStringValue(row, merchantNoIdx), "/ "),
			Remark:        strings.Trim(getStringValue(row, remarkIdx), "/ "),
			Category:      "其它",
		}

		// 解析金额
		if amountIdx >= 0 {
			amountStr := strings.TrimPrefix(row[amountIdx], "¥")
			fmt.Sscanf(amountStr, "%f", &record.Amount)
		}

		// 跳过已全额退款的记录
		if record.Status == "已全额退款" {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}

// processAlipayBill 处理支付宝账单
func processAlipayBill(filename string) ([]BillRecord, error) {
	var records []BillRecord

	// 打开文件并使用GBK解码器
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 支付宝账单是GBK编码
	gbkReader := transform.NewReader(file, simplifiedchinese.GBK.NewDecoder())
	reader := csv.NewReader(gbkReader)

	// 设置更宽松的读取选项
	reader.FieldsPerRecord = -1 // 允许不同数量的字段
	reader.LazyQuotes = true    // 宽松引号处理

	// 跳过前4行元数据
	for i := 0; i < 4; i++ {
		_, err := reader.Read()
		if err != nil {
			return nil, err
		}
	}

	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(lines) < 2 {
		return records, nil
	}

	// 获取标题行
	header := lines[0]

	// 找到各列索引
	dateIdx := getIndex(header, "交易创建时间")
	typeIdx := getIndex(header, "类型")
	counterpartyIdx := getIndex(header, "交易对方")
	itemIdx := getIndex(header, "商品名称")
	inOutIdx := getIndex(header, "收/支")
	amountIdx := getIndex(header, "金额（元）")
	statusIdx := getIndex(header, "交易状态")
	tradeNoIdx := getIndex(header, "交易号")
	merchantNoIdx := getIndex(header, "商家订单号")
	remarkIdx := getIndex(header, "备注")
	refundIdx := getIndex(header, "成功退款（元）")
	fundStatusIdx := getIndex(header, "资金状态")

	// 处理数据行
	for i := 1; i < len(lines); i++ {
		row := lines[i]
		// 检查行是否包含足够的字段
		if len(row) < max(dateIdx, typeIdx, counterpartyIdx, itemIdx, inOutIdx, amountIdx, statusIdx, tradeNoIdx, merchantNoIdx, remarkIdx, refundIdx, fundStatusIdx)+1 {
			continue
		}

		// 对于资金状态为空的情况，交易关闭，不计入结算
		if fundStatusIdx >= 0 && row[fundStatusIdx] == "" {
			continue
		}

		// 对于资金状态为"资金转移"的，如果服务费为零，不计入结算
		// TODO: 处理服务费逻辑

		// 跳过空的收支记录
		if inOutIdx >= 0 && (row[inOutIdx] == "" || row[inOutIdx] == "其他" || row[inOutIdx] == "不计收支") {
			continue
		}

		// 计算实际金额（金额 - 退款）
		var amount, refund float64

		if amountIdx >= 0 {
			amountStr := row[amountIdx]
			fmt.Sscanf(amountStr, "%f", &amount)
		}

		if refundIdx >= 0 {
			refundStr := row[refundIdx]
			fmt.Sscanf(refundStr, "%f", &refund)
		}

		actualAmount := amount - refund

		// 跳过金额为0的记录
		if actualAmount == 0 {
			continue
		}

		record := BillRecord{
			Date:         getStringValue(row, dateIdx),
			AccountType:  "支付宝",
			Type:         getStringValue(row, typeIdx),
			Counterparty: strings.Trim(getStringValue(row, counterpartyIdx), " "),
			ItemName:     strings.Trim(getStringValue(row, itemIdx), " "),
			InOut:        getStringValue(row, inOutIdx),
			Amount:       actualAmount,
			Status:       strings.Trim(getStringValue(row, statusIdx), " "),
			TradeNo:      strings.Trim(getStringValue(row, tradeNoIdx), " "),
			MerchantNo:   strings.Trim(getStringValue(row, merchantNoIdx), " "),
			Remark:       strings.Trim(getStringValue(row, remarkIdx), " "),
			Category:     "其它",
		}

		records = append(records, record)
	}

	return records, nil
}

// getIndex 获取指定列名在标题中的索引
func getIndex(header []string, columnName string) int {
	for i, col := range header {
		if strings.TrimSpace(col) == columnName {
			return i
		}
	}
	return -1
}

// getStringValue 安全地获取字符串值
func getStringValue(row []string, index int) string {
	if index >= 0 && index < len(row) {
		return row[index]
	}
	return ""
}

// getDateValue 安全地获取日期值
func getDateValue(row []string, index int) string {
	if index >= 0 && index < len(row) {
		return row[index]
	}
	return ""
}

// max 返回整数中的最大值
func max(values ...int) int {
	maxValue := values[0]
	for _, v := range values {
		if v > maxValue {
			maxValue = v
		}
	}
	return maxValue
}

// deduplicateRecords 去除重复记录
func deduplicateRecords(records []BillRecord) []BillRecord {
	seen := make(map[string]bool)
	result := make([]BillRecord, 0, len(records))

	for _, record := range records {
		// 创建唯一标识符
		key := fmt.Sprintf("%s|%s|%s|%s|%f",
			record.Date,
			record.Counterparty,
			record.ItemName,
			record.InOut,
			record.Amount)

		if !seen[key] {
			seen[key] = true
			result = append(result, record)
		}
	}

	return result
}

// saveMonthlyBills 按月份保存账单
func saveMonthlyBills(records []BillRecord) error {
	// 创建结果目录
	err := os.MkdirAll("result", 0o755)
	if err != nil {
		return err
	}

	// 按月份分组
	monthlyRecords := make(map[string][]BillRecord)
	for _, record := range records {
		// 解析日期以获取年月
		if len(record.Date) >= 7 {
			month := record.Date[:7] // YYYY-MM
			monthlyRecords[month] = append(monthlyRecords[month], record)
		}
	}

	// 为每个月份创建账单文件
	for month, monthRecords := range monthlyRecords {
		// 计算该月的第一天和最后一天
		startDate := fmt.Sprintf("%s-01", month)

		// 解析日期以计算月末
		t, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			continue
		}

		// 计算月末日期
		nextMonth := t.AddDate(0, 1, 0)
		lastDay := nextMonth.AddDate(0, 0, -1)
		endDate := lastDay.Format("2006-01-02")

		filename := fmt.Sprintf("result/%s~%s", startDate[:7], endDate[:7])

		// 保存为CSV
		err = saveAsCSV(fmt.Sprintf("%s.csv", filename), monthRecords)
		if err != nil {
			fmt.Printf("保存 %s.csv 失败: %v\n", filename, err)
		} else {
			fmt.Printf("已保存 %s.csv (%d 条记录)\n", filename, len(monthRecords))
		}

		// 保存为XLSX
		err = saveAsXLSX(fmt.Sprintf("%s.xlsx", filename), monthRecords)
		if err != nil {
			fmt.Printf("保存 %s.xlsx 失败: %v\n", filename, err)
		} else {
			fmt.Printf("已保存 %s.xlsx (%d 条记录)\n", filename, len(monthRecords))
		}
	}

	return nil
}

// saveAllBills 保存总表
func saveAllBills(records []BillRecord) error {
	// 创建结果目录
	err := os.MkdirAll("result", 0o755)
	if err != nil {
		return err
	}

	// 保存为XLSX
	err = saveAsXLSX("result/all.xlsx", records)
	if err != nil {
		return err
	}

	fmt.Printf("已保存总表 all.xlsx (%d 条记录)\n", len(records))
	return nil
}

// generateMonthlySummary 生成月度汇总
func generateMonthlySummary(records []BillRecord) []MonthlySummary {
	// 按月份分组
	monthlyData := make(map[string]*MonthlySummary)
	for _, record := range records {
		// 解析日期以获取年月
		if len(record.Date) >= 7 {
			month := record.Date[:7] // YYYY-MM

			// 初始化月度数据
			if _, exists := monthlyData[month]; !exists {
				monthlyData[month] = &MonthlySummary{
					Month:   month,
					Income:  0,
					Expense: 0,
					Records: 0,
				}
			}

			// 累计金额
			if record.InOut == "收入" {
				monthlyData[month].Income += record.Amount
			} else if record.InOut == "支出" {
				monthlyData[month].Expense += record.Amount
			}

			// 增加记录数
			monthlyData[month].Records++
		}
	}

	// 转换为切片
	var summary []MonthlySummary
	for _, s := range monthlyData {
		summary = append(summary, *s)
	}

	// 按月份排序
	// 简单的冒泡排序
	for i := 0; i < len(summary)-1; i++ {
		for j := 0; j < len(summary)-i-1; j++ {
			if summary[j].Month > summary[j+1].Month {
				summary[j], summary[j+1] = summary[j+1], summary[j]
			}
		}
	}

	return summary
}

// saveMonthlySummary 保存月度汇总
func saveMonthlySummary(summary []MonthlySummary) error {
	// 创建结果目录
	err := os.MkdirAll("result", 0o755)
	if err != nil {
		return err
	}

	// 保存为CSV
	file, err := os.Create("result/monthly_summary.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入标题行
	header := []string{"月份", "收入", "支出", "净收入", "记录数"}
	err = writer.Write(header)
	if err != nil {
		return err
	}

	// 写入数据行
	for _, s := range summary {
		row := []string{
			s.Month,
			fmt.Sprintf("%.2f", s.Income),
			fmt.Sprintf("%.2f", s.Expense),
			fmt.Sprintf("%.2f", s.Income-s.Expense),
			fmt.Sprintf("%d", s.Records),
		}
		err = writer.Write(row)
		if err != nil {
			return err
		}
	}

	fmt.Printf("已保存月度汇总 monthly_summary.csv (%d 条记录)\n", len(summary))
	return nil
}

// saveAsCSV 保存为CSV格式
func saveAsCSV(filename string, records []BillRecord) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入标题行
	header := []string{"时间", "账户1", "类型", "支付状态", "交易类型", "交易对方", "备注", "金额", "分类"}
	err = writer.Write(header)
	if err != nil {
		return err
	}

	// 写入数据行
	for _, record := range records {
		row := []string{
			record.Date,
			record.AccountType,
			record.InOut,
			record.Status,
			record.Type,
			record.Counterparty,
			record.Remark,
			fmt.Sprintf("%.2f", record.Amount),
			record.Category,
		}
		err = writer.Write(row)
		if err != nil {
			return err
		}
	}

	return nil
}

// saveAsXLSX 保存为XLSX格式
func saveAsXLSX(filename string, records []BillRecord) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// 创建工作表
	index, err := f.NewSheet("账单")
	if err != nil {
		return err
	}

	// 设置标题行
	titles := []string{"时间", "账户1", "类型", "支付状态", "交易类型", "交易对方", "备注", "金额", "分类"}
	for i, title := range titles {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("账单", cell, title)
	}

	// 写入数据行
	for i, record := range records {
		rowNum := i + 2 // 从第二行开始
		data := []interface{}{
			record.Date,
			record.AccountType,
			record.InOut,
			record.Status,
			record.Type,
			record.Counterparty,
			record.Remark,
			record.Amount,
			record.Category,
		}

		for j, value := range data {
			cell, _ := excelize.CoordinatesToCellName(j+1, rowNum)
			f.SetCellValue("账单", cell, value)
		}
	}

	// 设置工作表为活动工作表
	f.SetActiveSheet(index)

	// 保存文件
	if err := f.SaveAs(filename); err != nil {
		return err
	}

	return nil
}
