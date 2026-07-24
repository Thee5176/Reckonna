package domain

import "testing"

// AT-B2: unit coverage for the Money value object — plan 03 §1 (money paths must
// never touch float64; every money path includes an invalid case).
func TestMoneyFromString(t *testing.T) {
	m, err := MoneyFromString("1000.3333")
	if err != nil {
		t.Fatalf("MoneyFromString valid: unexpected error: %v", err)
	}
	if m.String() != "1000.3333" {
		t.Fatalf("String() = %q, want 1000.3333", m.String())
	}
	// INVALID case (mandatory for money/domain): a non-decimal literal → error.
	if _, err := MoneyFromString("not-money"); err == nil {
		t.Fatal("MoneyFromString(non-decimal) returned nil error, want error")
	}
}

func TestMoneyPredicatesAndOps(t *testing.T) {
	neg, _ := MoneyFromString("-0.01")
	zero, _ := MoneyFromString("0.0000")
	pos, _ := MoneyFromString("100")
	hundred, _ := MoneyFromString("100.00") // scale-insensitive equality

	if !neg.IsNegative() {
		t.Error("IsNegative(-0.01) = false, want true")
	}
	if pos.IsNegative() {
		t.Error("IsNegative(100) = true, want false")
	}
	if !zero.IsZero() {
		t.Error("IsZero(0) = false, want true")
	}
	if pos.IsZero() {
		t.Error("IsZero(100) = true, want false")
	}
	if !pos.Add(zero).Equal(hundred) {
		t.Error("100 + 0 not Equal 100.00 (scale-insensitive)")
	}
	if pos.Equal(neg) {
		t.Error("100 Equal -0.01, want not equal")
	}
	if got := NewMoney(pos.Decimal()); !got.Equal(pos) {
		t.Error("NewMoney(pos.Decimal()) not Equal pos")
	}
}
