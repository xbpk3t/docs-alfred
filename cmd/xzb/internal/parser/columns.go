package parser

// CSV column names for Alipay and WeChat transaction exports.
// These must match the header row exactly.
const (
	// Shared between Alipay and WeChat.
	colCounterparty = "交易对方"
	colInOut        = "收/支"
	colAmountCN     = "金额（元）"
	colAmountCNAlt  = "金额(元)"

	// Alipay-specific.
	colTradeNo         = "交易号"
	colTradeCreateTime = "交易创建时间"
	colTradeType       = "类型"
	colItemName        = "商品名称"
	colTradeStatus     = "交易状态"

	// WeChat-specific.
	colTradeTime     = "交易时间"
	colTransactionType = "交易类型"
	colCurrentStatus = "当前状态"
	colTradeOrderNo  = "交易单号"
)
