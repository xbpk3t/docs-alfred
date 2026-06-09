package aggregate

import (
	"sort"

	"github.com/xbpk3t/docs-alfred/xzb/internal/model"
)

type Summary struct {
	Months            []MonthSummary    `json:"months"`
	Categories        []CategorySummary `json:"categories"`
	TotalIncomeCents  int64             `json:"totalIncomeCents"`
	TotalExpenseCents int64             `json:"totalExpenseCents"`
	BudgetCents       int64             `json:"budgetCents"`
	Records           int               `json:"records"`
}

type MonthSummary struct {
	Month            string `json:"month"`
	IncomeCents      int64  `json:"incomeCents"`
	ExpenseCents     int64  `json:"expenseCents"`
	BudgetCents      int64  `json:"budgetCents"`
	TransactionCount int    `json:"transactionCount"`
}

type CategorySummary struct {
	Category         string `json:"category"`
	AmountCents      int64  `json:"amountCents"`
	TransactionCount int    `json:"transactionCount"`
}

func Build(transactions []model.Transaction) Summary {
	monthMap := make(map[string]*MonthSummary)
	categoryMap := make(map[string]*CategorySummary)
	summary := Summary{Records: len(transactions)}

	for i := range transactions {
		t := &transactions[i]
		month := monthMap[t.Month]
		if month == nil {
			month = &MonthSummary{Month: t.Month}
			monthMap[t.Month] = month
		}
		month.TransactionCount++

		switch t.InOut {
		case "收入":
			summary.TotalIncomeCents += t.AmountCents
			month.IncomeCents += t.AmountCents
		case "支出":
			summary.TotalExpenseCents += t.AmountCents
			month.ExpenseCents += t.AmountCents
			if t.BudgetIncluded {
				summary.BudgetCents += t.AmountCents
				month.BudgetCents += t.AmountCents
				category := categoryMap[t.Category]
				if category == nil {
					category = &CategorySummary{Category: t.Category}
					categoryMap[t.Category] = category
				}
				category.AmountCents += t.AmountCents
				category.TransactionCount++
			}
		}
	}

	for _, month := range monthMap {
		summary.Months = append(summary.Months, *month)
	}
	sort.Slice(summary.Months, func(i, j int) bool { return summary.Months[i].Month < summary.Months[j].Month })

	for _, category := range categoryMap {
		summary.Categories = append(summary.Categories, *category)
	}
	sort.Slice(summary.Categories, func(i, j int) bool {
		if summary.Categories[i].AmountCents == summary.Categories[j].AmountCents {
			return summary.Categories[i].Category < summary.Categories[j].Category
		}

		return summary.Categories[i].AmountCents > summary.Categories[j].AmountCents
	})

	return summary
}
