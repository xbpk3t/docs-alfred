package model

import "time"

type Source string

const (
	SourceWechat Source = "wechat"
	SourceAlipay Source = "alipay"
)

type Transaction struct {
	OccurredAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ID              string
	Source          Source
	SourceFile      string
	SourceTradeNo   string
	MerchantTradeNo string
	Month           string
	AccountType     string
	InOut           string
	TransactionType string
	Counterparty    string
	ItemName        string
	PaymentMethod   string
	Status          string
	Remark          string
	Category        string
	BudgetRule      string
	ImportBatchID   string
	AmountCents     int64
	BudgetIncluded  bool
}

type ParsedTransaction struct {
	OccurredAt      time.Time
	Source          Source
	SourceFile      string
	SourceTradeNo   string
	MerchantTradeNo string
	AccountType     string
	InOut           string
	TransactionType string
	Counterparty    string
	ItemName        string
	PaymentMethod   string
	Status          string
	Remark          string
	AmountCents     int64
}

func (t *ParsedTransaction) Normalize(now time.Time) Transaction {
	return Transaction{
		OccurredAt:      t.OccurredAt,
		CreatedAt:       now,
		UpdatedAt:       now,
		Source:          t.Source,
		SourceFile:      t.SourceFile,
		SourceTradeNo:   t.SourceTradeNo,
		MerchantTradeNo: t.MerchantTradeNo,
		Month:           t.OccurredAt.Format("2006-01"),
		AccountType:     t.AccountType,
		InOut:           t.InOut,
		TransactionType: t.TransactionType,
		Counterparty:    t.Counterparty,
		ItemName:        t.ItemName,
		PaymentMethod:   t.PaymentMethod,
		Status:          t.Status,
		Remark:          t.Remark,
		AmountCents:     t.AmountCents,
	}
}
