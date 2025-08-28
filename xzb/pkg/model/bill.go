// Package model provides data models for bill records
package model

// BillRecord represents a bill record
type BillRecord struct {
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
	Amount        float64 // Amount
}
