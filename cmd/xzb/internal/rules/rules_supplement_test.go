package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

func TestValidateVersion(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    2,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "c", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "unsupported rules version")
}

func TestValidateMissingCategory(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "c", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "defaults.category is required")
}

func TestValidateMissingBudgetIncluded(t *testing.T) {
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat"},
		Categories: []Category{{Name: "c", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "defaults.budgetIncluded is required")
}

func TestValidateCategoryMissingName(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "categories[0].name is required")
}

func TestValidateBudgetRuleMissingName(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		BudgetRules: []BudgetRule{{Name: "", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "budgetRules[0].name is required")
}

func TestValidateMatchEmpty(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "c", Match: Match{}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "any or all is required")
}

func TestValidateConditionBothEqualsAndContains(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "c", Match: Match{Any: []Condition{{Field: "inOut", Equals: "支出", Contains: "支"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "cannot use both equals and contains")
}

func TestValidateConditionMissingEqualsAndContains(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		Categories: []Category{{Name: "c", Match: Match{Any: []Condition{{Field: "inOut"}}}}},
	}
	require.Error(t, cfg.Validate())
	require.Contains(t, cfg.Validate().Error(), "needs equals or contains")
}

func TestMatchAllLogic(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "default", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "fast food", Match: Match{All: []Condition{
				{Field: "inOut", Equals: "支出"},
				{Field: "counterparty", Contains: "麦当劳"},
			}}},
		},
	}

	tx := &model.Transaction{InOut: "支出", Counterparty: "麦当劳"}
	require.Equal(t, "fast food", cfg.CategoryFor(tx))

	tx2 := &model.Transaction{InOut: "收入", Counterparty: "麦当劳"}
	require.Equal(t, "default", cfg.CategoryFor(tx2))

	tx3 := &model.Transaction{InOut: "支出", Counterparty: "肯德基"}
	require.Equal(t, "default", cfg.CategoryFor(tx3))
}

func TestMatchAnyLogic(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "default", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "food", Match: Match{Any: []Condition{
				{Field: "counterparty", Contains: "麦当劳"},
				{Field: "counterparty", Contains: "肯德基"},
			}}},
		},
	}

	tx := &model.Transaction{Counterparty: "麦当劳"}
	require.Equal(t, "food", cfg.CategoryFor(tx))

	tx2 := &model.Transaction{Counterparty: "肯德基"}
	require.Equal(t, "food", cfg.CategoryFor(tx2))

	tx3 := &model.Transaction{Counterparty: "星巴克"}
	require.Equal(t, "default", cfg.CategoryFor(tx3))
}

func TestCategoryForFirstMatch(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "default", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "first", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "test"}}}},
			{Name: "second", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "test"}}}},
		},
	}
	tx := &model.Transaction{Counterparty: "test"}
	require.Equal(t, "first", cfg.CategoryFor(tx))
}

func TestCategoryForDefault(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "unclassified", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "food", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "麦当劳"}}}},
		},
	}
	tx := &model.Transaction{Counterparty: "星巴克"}
	require.Equal(t, "unclassified", cfg.CategoryFor(tx))
}

func TestBudgetForFirstMatch(t *testing.T) {
	boolFalse := false
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolFalse},
		BudgetRules: []BudgetRule{
			{Name: "exclude-income", BudgetIncluded: false, Match: Match{Any: []Condition{{Field: "inOut", Equals: "收入"}}}},
			{Name: "include-expense", BudgetIncluded: true, Match: Match{All: []Condition{{Field: "inOut", Equals: "支出"}}}},
		},
	}

	decision := cfg.budgetFor(&model.Transaction{InOut: "收入"})
	require.False(t, decision.included)
	require.Equal(t, "exclude-income", decision.rule)

	decision2 := cfg.budgetFor(&model.Transaction{InOut: "支出"})
	require.True(t, decision2.included)
	require.Equal(t, "include-expense", decision2.rule)
}

func TestBudgetForDefault(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
		BudgetRules: []BudgetRule{
			{Name: "exclude-income", BudgetIncluded: false, Match: Match{Any: []Condition{{Field: "inOut", Equals: "收入"}}}},
		},
	}

	decision := cfg.budgetFor(&model.Transaction{InOut: "支出"})
	require.True(t, decision.included)
	require.Equal(t, "default", decision.rule)
}

func TestApplySetsCategoryAndBudget(t *testing.T) {
	boolTrue := true
	cfg := &Config{
		Version:    1,
		Defaults:   Defaults{Category: "other", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "food", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "麦当劳"}}}},
		},
		BudgetRules: []BudgetRule{
			{Name: "no-income", BudgetIncluded: false, Match: Match{Any: []Condition{{Field: "inOut", Equals: "收入"}}}},
		},
	}

	transactions := []model.Transaction{
		{Counterparty: "麦当劳", InOut: "支出"},
		{Counterparty: "公司", InOut: "收入"},
	}

	result := Apply(cfg, transactions)
	require.Len(t, result, 2)
	require.Equal(t, "food", result[0].Category)
	require.True(t, result[0].BudgetIncluded)
	require.Equal(t, "other", result[1].Category)
	require.False(t, result[1].BudgetIncluded)
	require.Equal(t, "no-income", result[1].BudgetRule)
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	boolTrue := true
	cfg := &Config{
		Version:    1,
		Defaults:   Defaults{Category: "cat", BudgetIncluded: &boolTrue},
	}
	input := []model.Transaction{{InOut: "支出"}}
	result := Apply(cfg, input)
	require.Equal(t, "cat", result[0].Category)
	require.Empty(t, input[0].Category)
}

func TestFieldKnown(t *testing.T) {
	require.True(t, fieldKnown("source"))
	require.True(t, fieldKnown("counterparty"))
	require.True(t, fieldKnown("month"))
	require.False(t, fieldKnown("unknown"))
	require.False(t, fieldKnown(""))
}

func TestFieldValue(t *testing.T) {
	tx := &model.Transaction{Counterparty: "test", Source: model.SourceWechat}
	v, ok := fieldValue("counterparty", tx)
	require.True(t, ok)
	require.Equal(t, "test", v)

	v2, ok2 := fieldValue("source", tx)
	require.True(t, ok2)
	require.Equal(t, "wechat", v2)

	_, ok3 := fieldValue("unknown", tx)
	require.False(t, ok3)

	_, ok4 := fieldValue("counterparty", nil)
	require.False(t, ok4)
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yml")
	content := `version: 1
defaults:
  category: "未分类"
  budgetIncluded: true
categories:
  - name: "餐饮"
    match:
      any:
        - field: counterparty
          contains: "麦当劳"
budgetRules:
  - name: "exclude-income"
    budgetIncluded: false
    match:
      any:
        - field: inOut
          equals: "收入"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 1, cfg.Version)
	require.Equal(t, "未分类", cfg.Defaults.Category)
	require.Len(t, cfg.Categories, 1)
	require.Len(t, cfg.BudgetRules, 1)
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/rules.yml")
	require.Error(t, err)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid yaml"), 0600))

	_, err := Load(path)
	require.Error(t, err)
}

func TestLoadInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yml")
	content := `version: 99
defaults:
  category: "cat"
  budgetIncluded: true
categories: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported rules version")
}

func TestAllFieldGetters(t *testing.T) {
	tx := &model.Transaction{
		Source:          model.SourceWechat,
		SourceFile:      "test.csv",
		SourceTradeNo:   "trade-1",
		MerchantTradeNo: "merch-1",
		AccountType:     "微信",
		InOut:           "支出",
		TransactionType: "商户消费",
		Counterparty:    "商户",
		ItemName:        "商品",
		PaymentMethod:   "零钱",
		Status:          "支付成功",
		Remark:          "备注",
		Category:        "餐饮",
		Month:           "2026-05",
	}
	tests := []struct {
		field string
		want  string
	}{
		{"source", "wechat"},
		{"sourceFile", "test.csv"},
		{"sourceTradeNo", "trade-1"},
		{"merchantTradeNo", "merch-1"},
		{"accountType", "微信"},
		{"inOut", "支出"},
		{"transactionType", "商户消费"},
		{"counterparty", "商户"},
		{"itemName", "商品"},
		{"paymentMethod", "零钱"},
		{"status", "支付成功"},
		{"remark", "备注"},
		{"category", "餐饮"},
		{"month", "2026-05"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			v, ok := fieldValue(tt.field, tx)
			require.True(t, ok)
			require.Equal(t, tt.want, v)
		})
	}
}

func TestContainsMatch(t *testing.T) {
	boolTrue := true
	cfg := Config{
		Version:    1,
		Defaults:   Defaults{Category: "default", BudgetIncluded: &boolTrue},
		Categories: []Category{
			{Name: "food", Match: Match{Any: []Condition{{Field: "counterparty", Contains: "麦"}}}},
		},
	}

	require.Equal(t, "food", cfg.CategoryFor(&model.Transaction{Counterparty: "麦当劳"}))
	require.Equal(t, "default", cfg.CategoryFor(&model.Transaction{Counterparty: "肯德基"}))
}
