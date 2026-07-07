package domain

import "testing"

// AT-B2: AccountType.Valid + DimensionType.Valid — plan 03 §1 (CoA §6 / dimensions §7).
func TestAccountTypeValid(t *testing.T) {
	for _, ty := range []AccountType{AccountAsset, AccountLiability, AccountEquity, AccountIncome, AccountExpense} {
		if !ty.Valid() {
			t.Errorf("AccountType(%q).Valid() = false, want true", ty)
		}
	}
	if AccountType("bogus").Valid() {
		t.Error("AccountType(bogus).Valid() = true, want false")
	}
}

func TestDimensionTypeValid(t *testing.T) {
	for _, d := range []DimensionType{DimEntity, DimCurrency, DimCounterparty} {
		if !d.Valid() {
			t.Errorf("DimensionType(%q).Valid() = false, want true", d)
		}
	}
	if DimensionType("bogus").Valid() {
		t.Error("DimensionType(bogus).Valid() = true, want false")
	}
}
