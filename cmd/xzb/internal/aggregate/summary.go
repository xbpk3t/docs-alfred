package aggregate

import (
	"sort"

	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

const (
	inOutIncome  = "收入"
	inOutExpense = "支出"
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
	categoryMap := make(map[string]*CategorySummary)
	summary := Summary{Records: len(transactions)}

	groups := lo.GroupBy(transactions, func(t model.Transaction) string { return t.Month })
	monthNames := lo.Keys(groups)
	sort.Strings(monthNames)

	for _, monthKey := range monthNames {
		txns := groups[monthKey]
		month := MonthSummary{Month: monthKey}
		for i := range txns {
			t := &txns[i]
			month.TransactionCount++

			switch t.InOut {
			case inOutIncome:
				summary.TotalIncomeCents += t.AmountCents
				month.IncomeCents += t.AmountCents
			case inOutExpense:
				summary.TotalExpenseCents += t.AmountCents
				month.ExpenseCents += t.AmountCents
				if t.BudgetIncluded {
					summary.BudgetCents += t.AmountCents
					month.BudgetCents += t.AmountCents
					cat := categoryMap[t.Category]
					if cat == nil {
						cat = &CategorySummary{Category: t.Category}
						categoryMap[t.Category] = cat
					}
					cat.AmountCents += t.AmountCents
					cat.TransactionCount++
				}
			}
		}
		summary.Months = append(summary.Months, month)
	}

	for _, cat := range categoryMap {
		summary.Categories = append(summary.Categories, *cat)
	}
	sort.Slice(summary.Categories, func(i, j int) bool {
		if summary.Categories[i].AmountCents == summary.Categories[j].AmountCents {
			return summary.Categories[i].Category < summary.Categories[j].Category
		}

		return summary.Categories[i].AmountCents > summary.Categories[j].AmountCents
	})

	return summary
}
