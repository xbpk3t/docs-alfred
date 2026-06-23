package aggregate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

func TestBuildEmpty(t *testing.T) {
	s := Build(nil)
	require.Equal(t, 0, s.Records)
	require.Empty(t, s.Months)
	require.Empty(t, s.Categories)
	require.Equal(t, int64(0), s.TotalIncomeCents)
	require.Equal(t, int64(0), s.TotalExpenseCents)
	require.Equal(t, int64(0), s.BudgetCents)
}

func TestBuildIncomeExpense(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-05", InOut: "收入", AmountCents: 100000},
		{Month: "2026-05", InOut: "支出", AmountCents: 3500, BudgetIncluded: true, Category: "餐饮"},
		{Month: "2026-05", InOut: "支出", AmountCents: 2000, BudgetIncluded: false, Category: "娱乐"},
	}
	s := Build(txns)

	require.Equal(t, 3, s.Records)
	require.Equal(t, int64(100000), s.TotalIncomeCents)
	require.Equal(t, int64(5500), s.TotalExpenseCents)
	require.Equal(t, int64(3500), s.BudgetCents)
	require.Len(t, s.Months, 1)
	require.Equal(t, "2026-05", s.Months[0].Month)
	require.Equal(t, int64(100000), s.Months[0].IncomeCents)
	require.Equal(t, int64(5500), s.Months[0].ExpenseCents)
	require.Equal(t, int64(3500), s.Months[0].BudgetCents)
	require.Equal(t, 3, s.Months[0].TransactionCount)
}

func TestBuildCategoriesSortedByAmount(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-05", InOut: "支出", AmountCents: 1000, BudgetIncluded: true, Category: "餐饮"},
		{Month: "2026-05", InOut: "支出", AmountCents: 5000, BudgetIncluded: true, Category: "购物"},
		{Month: "2026-05", InOut: "支出", AmountCents: 1000, BudgetIncluded: true, Category: "交通"},
		{Month: "2026-05", InOut: "支出", AmountCents: 3000, BudgetIncluded: true, Category: "餐饮"},
	}
	s := Build(txns)

	require.Len(t, s.Categories, 3)
	// Descending by amount
	require.Equal(t, "购物", s.Categories[0].Category)
	require.Equal(t, int64(5000), s.Categories[0].AmountCents)
	require.Equal(t, 1, s.Categories[0].TransactionCount)
	require.Equal(t, "餐饮", s.Categories[1].Category)
	require.Equal(t, int64(4000), s.Categories[1].AmountCents)
	require.Equal(t, 2, s.Categories[1].TransactionCount)
	require.Equal(t, "交通", s.Categories[2].Category)
	require.Equal(t, int64(1000), s.Categories[2].AmountCents)
}

func TestBuildCategoriesSameAmountSortedByName(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-05", InOut: "支出", AmountCents: 1000, BudgetIncluded: true, Category: "Banana"},
		{Month: "2026-05", InOut: "支出", AmountCents: 1000, BudgetIncluded: true, Category: "Apple"},
	}
	s := Build(txns)

	require.Len(t, s.Categories, 2)
	require.Equal(t, "Apple", s.Categories[0].Category)
	require.Equal(t, "Banana", s.Categories[1].Category)
}

func TestBuildMultipleMonthsSorted(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-06", InOut: "支出", AmountCents: 1000, BudgetIncluded: true, Category: "餐饮"},
		{Month: "2026-04", InOut: "收入", AmountCents: 50000},
		{Month: "2026-05", InOut: "支出", AmountCents: 2000, BudgetIncluded: true, Category: "餐饮"},
	}
	s := Build(txns)

	require.Len(t, s.Months, 3)
	require.Equal(t, "2026-04", s.Months[0].Month)
	require.Equal(t, "2026-05", s.Months[1].Month)
	require.Equal(t, "2026-06", s.Months[2].Month)
}

func TestBuildIgnoresNonIncomeNonExpense(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-05", InOut: "其他", AmountCents: 1000},
		{Month: "2026-05", InOut: "", AmountCents: 2000},
	}
	s := Build(txns)

	require.Equal(t, int64(0), s.TotalIncomeCents)
	require.Equal(t, int64(0), s.TotalExpenseCents)
	require.Len(t, s.Months, 1)
	require.Equal(t, int64(0), s.Months[0].IncomeCents)
	require.Equal(t, int64(0), s.Months[0].ExpenseCents)
}

func TestBuildNonBudgetExpenseNotInCategories(t *testing.T) {
	txns := []model.Transaction{
		{Month: "2026-05", InOut: "支出", AmountCents: 5000, BudgetIncluded: false, Category: "娱乐"},
	}
	s := Build(txns)

	require.Equal(t, int64(5000), s.TotalExpenseCents)
	require.Equal(t, int64(0), s.BudgetCents)
	require.Empty(t, s.Categories)
}
