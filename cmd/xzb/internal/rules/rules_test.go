package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

func TestRulesFirstMatch(t *testing.T) {
	budgetDefault := true
	cfg := Config{
		Version:  1,
		Defaults: Defaults{Category: "未分类", BudgetIncluded: &budgetDefault},
		Categories: []Category{
			{Name: "餐饮", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "麦当劳"}}}},
			{Name: "快餐", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "麦当劳"}}}},
		},
		BudgetRules: []BudgetRule{
			{Name: "exclude-income", BudgetIncluded: false, Match: Match{Any: []Condition{{Field: "inOut", Equals: "收入"}}}},
			{Name: "include-expense", BudgetIncluded: true, Match: Match{All: []Condition{{Field: "inOut", Equals: "支出"}}}},
		},
	}
	require.NoError(t, cfg.Validate())

	transactions := Apply(&cfg, []model.Transaction{{InOut: "支出", Counterparty: "麦当劳"}, {InOut: "收入", Counterparty: "公司"}})
	require.Equal(t, "餐饮", transactions[0].Category)
	require.True(t, transactions[0].BudgetIncluded)
	require.Equal(t, "include-expense", transactions[0].BudgetRule)
	require.Equal(t, "未分类", transactions[1].Category)
	require.False(t, transactions[1].BudgetIncluded)
	require.Equal(t, "exclude-income", transactions[1].BudgetRule)
}

func TestValidateUnknownField(t *testing.T) {
	budgetDefault := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "未分类", BudgetIncluded: &budgetDefault},
		Categories: []Category{{Name: "bad", Match: Match{Any: []Condition{{Field: "rawColumn", Contains: "x"}}}}},
	}
	require.Error(t, cfg.Validate())
}
