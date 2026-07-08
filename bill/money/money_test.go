package money

import (
	"errors"
	"math"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"10.50", 1050},
		{"10.5", 1050},
		{"0.01", 1},
		{"3", 300},
		{"0", 0},
		{"0.00", 0},
	}
	for _, c := range cases {
		got, err := ParseDecimal(USD, c.in)
		if err != nil {
			t.Fatalf("ParseDecimal(%q) unexpected error: %v", c.in, err)
		}
		if got.MinorUnits != c.want {
			t.Errorf("ParseDecimal(%q) = %d, want %d", c.in, got.MinorUnits, c.want)
		}
	}
}

func TestParseDecimalRejectsInvalid(t *testing.T) {
	cases := []string{"", "-1.00", "1.234", "abc", "1.", ".5", "1.2.3", "1,50"}
	for _, in := range cases {
		if _, err := ParseDecimal(USD, in); err == nil {
			t.Errorf("ParseDecimal(%q) expected error, got nil", in)
		}
	}
}

func TestParseDecimalUnsupportedCurrency(t *testing.T) {
	_, err := ParseDecimal("XXX", "10.00")
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("expected ErrUnsupportedCurrency, got %v", err)
	}
}

func TestDecimalStringRoundTrip(t *testing.T) {
	m, err := New(GEL, 1050)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.DecimalString(); got != "10.50" {
		t.Errorf("DecimalString() = %q, want %q", got, "10.50")
	}
}

func TestAdd(t *testing.T) {
	a, _ := New(USD, 1000)
	b, _ := New(USD, 250)
	sum, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	if sum.MinorUnits != 1250 {
		t.Errorf("sum = %d, want 1250", sum.MinorUnits)
	}
}

func TestAddCurrencyMismatch(t *testing.T) {
	usd, _ := New(USD, 100)
	gel, _ := New(GEL, 100)
	_, err := usd.Add(gel)
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestAddOverflow(t *testing.T) {
	a, _ := New(USD, math.MaxInt64)
	b, _ := New(USD, 1)
	_, err := a.Add(b)
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected ErrOverflow, got %v", err)
	}
}

func TestNewRejectsNegative(t *testing.T) {
	if _, err := New(USD, -1); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("expected ErrNegativeAmount, got %v", err)
	}
}
