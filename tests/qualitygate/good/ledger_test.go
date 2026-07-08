package good

import (
	"errors"
	"testing"
)

// Table-driven test — covers balanced, unbalanced, and empty cases.
// Ensures 借方=貸方 invariant has a failing branch covered.
func TestPostLedger(t *testing.T) {
	cases := []struct {
		name    string
		items   []LedgerItem
		want    int64
		wantErr error
	}{
		{"empty is balanced", nil, 0, nil},
		{"single balanced", []LedgerItem{{Debit: 100, Credit: 100}}, 100, nil},
		{"multi balanced", []LedgerItem{
			{Debit: 30, Credit: 0}, {Debit: 70, Credit: 0},
			{Debit: 0, Credit: 100},
		}, 100, nil},
		{"unbalanced rejected", []LedgerItem{
			{Debit: 100, Credit: 0}, {Debit: 0, Credit: 50},
		}, 0, ErrUnbalanced},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PostLedger(tc.items)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("total=%d want=%d", got, tc.want)
			}
		})
	}
}
