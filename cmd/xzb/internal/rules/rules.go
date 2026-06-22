package rules

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
)

type Config struct {
	Defaults    Defaults     `yaml:"defaults"`
	Categories  []Category   `yaml:"categories"`
	BudgetRules []BudgetRule `yaml:"budgetRules"`
	Version     int          `yaml:"version"`
}

type Defaults struct {
	BudgetIncluded *bool  `yaml:"budgetIncluded"`
	Category       string `yaml:"category"`
}

type Category struct {
	Name  string `yaml:"name"`
	Match Match  `yaml:"match"`
}

type BudgetRule struct {
	Name           string `yaml:"name"`
	Match          Match  `yaml:"match"`
	BudgetIncluded bool   `yaml:"budgetIncluded"`
}

type Match struct {
	Any []Condition `yaml:"any"`
	All []Condition `yaml:"all"`
}

type Condition struct {
	Field    string `yaml:"field"`
	Equals   string `yaml:"equals"`
	Contains string `yaml:"contains"`
}

var transactionFieldGetters = map[string]func(*model.Transaction) string{
	"source":          func(t *model.Transaction) string { return string(t.Source) },
	"sourceFile":      func(t *model.Transaction) string { return t.SourceFile },
	"sourceTradeNo":   func(t *model.Transaction) string { return t.SourceTradeNo },
	"merchantTradeNo": func(t *model.Transaction) string { return t.MerchantTradeNo },
	"accountType":     func(t *model.Transaction) string { return t.AccountType },
	"inOut":           func(t *model.Transaction) string { return t.InOut },
	"transactionType": func(t *model.Transaction) string { return t.TransactionType },
	"counterparty":    func(t *model.Transaction) string { return t.Counterparty },
	"itemName":        func(t *model.Transaction) string { return t.ItemName },
	"paymentMethod":   func(t *model.Transaction) string { return t.PaymentMethod },
	"status":          func(t *model.Transaction) string { return t.Status },
	"remark":          func(t *model.Transaction) string { return t.Remark },
	"category":        func(t *model.Transaction) string { return t.Category },
	"month":           func(t *model.Transaction) string { return t.Month },
}

type budgetDecision struct {
	rule     string
	included bool
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported rules version %d", c.Version)
	}
	if c.Defaults.Category == "" {
		return errors.New("defaults.category is required")
	}
	if c.Defaults.BudgetIncluded == nil {
		return errors.New("defaults.budgetIncluded is required")
	}
	for i := range c.Categories {
		rule := &c.Categories[i]
		if rule.Name == "" {
			return fmt.Errorf("categories[%d].name is required", i)
		}
		if err := validateMatch(rule.Match); err != nil {
			return fmt.Errorf("categories[%d].match: %w", i, err)
		}
	}
	for i := range c.BudgetRules {
		rule := &c.BudgetRules[i]
		if rule.Name == "" {
			return fmt.Errorf("budgetRules[%d].name is required", i)
		}
		if err := validateMatch(rule.Match); err != nil {
			return fmt.Errorf("budgetRules[%d].match: %w", i, err)
		}
	}

	return nil
}

func Apply(cfg *Config, transactions []model.Transaction) []model.Transaction {
	result := make([]model.Transaction, len(transactions))
	copy(result, transactions)
	for i := range result {
		transaction := &result[i]
		transaction.Category = cfg.CategoryFor(transaction)
		decision := cfg.budgetFor(transaction)
		transaction.BudgetIncluded = decision.included
		transaction.BudgetRule = decision.rule
	}

	return result
}

func (c *Config) CategoryFor(t *model.Transaction) string {
	for i := range c.Categories {
		rule := &c.Categories[i]
		if match(rule.Match, t) {
			return rule.Name
		}
	}

	return c.Defaults.Category
}

func (c *Config) budgetFor(t *model.Transaction) budgetDecision {
	for i := range c.BudgetRules {
		rule := &c.BudgetRules[i]
		if match(rule.Match, t) {
			return budgetDecision{included: rule.BudgetIncluded, rule: rule.Name}
		}
	}

	return budgetDecision{included: *c.Defaults.BudgetIncluded, rule: "default"}
}

func validateMatch(m Match) error {
	if len(m.Any) == 0 && len(m.All) == 0 {
		return errors.New("any or all is required")
	}
	if err := validateConditions(m.Any); err != nil {
		return err
	}
	if err := validateConditions(m.All); err != nil {
		return err
	}

	return nil
}

func validateConditions(conditions []Condition) error {
	for i := range conditions {
		condition := &conditions[i]
		if !fieldKnown(condition.Field) {
			return fmt.Errorf("unknown field %q", condition.Field)
		}
		if condition.Equals == "" && condition.Contains == "" {
			return fmt.Errorf("condition for %q needs equals or contains", condition.Field)
		}
		if condition.Equals != "" && condition.Contains != "" {
			return fmt.Errorf("condition for %q cannot use both equals and contains", condition.Field)
		}
	}

	return nil
}

func match(m Match, t *model.Transaction) bool {
	if len(m.Any) > 0 {
		for i := range m.Any {
			if matchCondition(&m.Any[i], t) {
				return true
			}
		}

		return false
	}

	for i := range m.All {
		if !matchCondition(&m.All[i], t) {
			return false
		}
	}

	return true
}

func matchCondition(c *Condition, t *model.Transaction) bool {
	value, ok := fieldValue(c.Field, t)
	if !ok {
		return false
	}
	if c.Equals != "" {
		return value == c.Equals
	}

	return strings.Contains(value, c.Contains)
}

func fieldKnown(field string) bool {
	_, ok := transactionFieldGetters[field]

	return ok
}

func fieldValue(field string, t *model.Transaction) (string, bool) {
	getter, ok := transactionFieldGetters[field]
	if !ok || t == nil {
		return "", false
	}

	return getter(t), true
}
