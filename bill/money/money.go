package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Currency string

const (
	USD Currency = "USD"
	GEL Currency = "GEL"
)

var exponents = map[Currency]int{
	USD: 2,
	GEL: 2,
}

var (
	ErrUnsupportedCurrency = errors.New("money: unsupported currency")
	ErrCurrencyMismatch    = errors.New("money: currency mismatch")
	ErrOverflow            = errors.New("money: amount overflow")
	ErrNegativeAmount      = errors.New("money: amount must not be negative")
	ErrInvalidAmount       = errors.New("money: invalid amount")
)

type Money struct {
	Currency   Currency `json:"currency"`
	MinorUnits int64    `json:"minorUnits"`
}

func Zero(c Currency) (Money, error) {
	if !IsSupported(c) {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}
	return Money{Currency: c, MinorUnits: 0}, nil
}

func New(c Currency, minorUnits int64) (Money, error) {
	if !IsSupported(c) {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}
	if minorUnits < 0 {
		return Money{}, ErrNegativeAmount
	}
	return Money{Currency: c, MinorUnits: minorUnits}, nil
}

func IsSupported(c Currency) bool {
	_, ok := exponents[c]
	return ok
}

func ParseDecimal(c Currency, s string) (Money, error) {
	exp, ok := exponents[c]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, c)
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return Money{}, fmt.Errorf("%w: empty amount", ErrInvalidAmount)
	}
	if strings.HasPrefix(s, "-") {
		return Money{}, ErrNegativeAmount
	}

	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	if !isDigits(whole) {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}
	if hasFrac {
		if !isDigits(frac) || len(frac) > exp {
			return Money{}, fmt.Errorf("%w: %q (max %d decimal places for %s)", ErrInvalidAmount, s, exp, c)
		}
		frac = frac + strings.Repeat("0", exp-len(frac))
	} else {
		frac = strings.Repeat("0", exp)
	}

	combined := whole + frac
	minorUnits, err := strconv.ParseInt(combined, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, s)
	}

	return Money{Currency: c, MinorUnits: minorUnits}, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	sum := m.MinorUnits + other.MinorUnits

	if sum < 0 {
		return Money{}, ErrOverflow
	}
	return Money{Currency: m.Currency, MinorUnits: sum}, nil
}

func (m Money) DecimalString() string {
	exp := exponents[m.Currency]
	if exp == 0 {
		return strconv.FormatInt(m.MinorUnits, 10)
	}
	s := strconv.FormatInt(m.MinorUnits, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	for len(s) <= exp {
		s = "0" + s
	}
	whole, frac := s[:len(s)-exp], s[len(s)-exp:]
	out := whole + "." + frac
	if neg {
		out = "-" + out
	}
	return out
}

func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.DecimalString(), m.Currency)
}
