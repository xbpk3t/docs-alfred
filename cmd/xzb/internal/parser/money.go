package parser

import (
	"fmt"
	"strconv"
	"strings"
)

func AmountCents(value string) (int64, error) {
	s := cleanAmountText(value)
	if s == "" || s == "/" {
		return 0, nil
	}

	unsigned, negative := stripAmountSign(s)
	parts, err := splitAmountParts(unsigned, value)
	if err != nil {
		return 0, err
	}
	yuan, err := strconv.ParseInt(parts.yuanText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}
	cents, err := strconv.ParseInt(parts.centsText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}

	total := yuan*100 + cents
	if negative {
		total = -total
	}

	return total, nil
}

func cleanAmountText(value string) string {
	s := strings.TrimSpace(value)
	s = strings.TrimPrefix(s, "\ufeff")
	replacer := strings.NewReplacer("¥", "", "￥", "", ",", "", " ", "", "\t", "")

	return replacer.Replace(s)
}

func stripAmountSign(value string) (string, bool) {
	if after, ok := strings.CutPrefix(value, "-"); ok {
		return after, true
	}

	return strings.TrimPrefix(value, "+"), false
}

type amountParts struct {
	yuanText  string
	centsText string
}

func splitAmountParts(value, original string) (amountParts, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return amountParts{}, fmt.Errorf("invalid amount %q", original)
	}
	if parts[0] == "" {
		parts[0] = "0"
	}
	if len(parts) == 1 {
		return amountParts{yuanText: parts[0], centsText: "00"}, nil
	}
	if len(parts[1]) > 2 {
		return amountParts{}, fmt.Errorf("amount %q has more than two decimal places", original)
	}
	if len(parts[1]) == 1 {
		parts[1] += "0"
	}

	return amountParts{yuanText: parts[0], centsText: parts[1]}, nil
}

func NonNegativeNetAmount(amountCents, refundCents int64) int64 {
	net := amountCents - refundCents
	if net < 0 {
		return 0
	}

	return net
}
