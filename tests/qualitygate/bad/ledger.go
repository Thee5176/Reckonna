// Package bad is a QG fixture. Every function intentionally violates a
// quality-gate condition. Do NOT import from production code.
package bad

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
)

// V1: cognitive complexity > 15 (nested branches + loops + switches).
func PostLedger(items []map[string]any, mode string) (int, error) {
	total := 0
	for _, it := range items {
		if it == nil {
			continue
		}
		switch mode {
		case "debit":
			if v, ok := it["amount"].(int); ok {
				if v > 0 {
					if v > 1000 {
						if v > 10000 {
							total += v * 2
						} else {
							total += v
						}
					} else {
						total += v / 2
					}
				} else if v < 0 {
					total -= v
				}
			}
		case "credit":
			if v, ok := it["amount"].(int); ok {
				if v > 0 {
					total -= v
				} else if v < -100 {
					total -= v * 3
				} else {
					total -= v / 2
				}
			}
		default:
			if v, ok := it["amount"].(int); ok {
				total += v
			}
		}
	}
	return total, nil
}

// V2: SQL injection vulnerability (string concat into Query).
func FindAccount(db *sql.DB, name string) (*sql.Rows, error) {
	q := fmt.Sprintf("SELECT id FROM accounts WHERE name = '%s'", name)
	return db.Query(q)
}

// V3: bug — error swallowed silently; nil-deref risk.
func SaveAmount(db *sql.DB, id string, amt int) int {
	row := db.QueryRow("SELECT balance FROM accounts WHERE id = $1", id)
	var b int
	row.Scan(&b)
	return b + amt
}

// V4: security hotspot — math/rand used for credential-like value.
func NewSessionToken() string {
	return fmt.Sprintf("tok-%d", rand.Int63())
}

// V5: naked panic outside main (violates project rule).
func MustBalance(d, c int) {
	if d != c {
		panic("unbalanced")
	}
}

// V6: duplicated block #1.
func HandleA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Service", "bad")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"ok":true,"route":"a"}`)
}

// V6: duplicated block #2 (same shape, trivial rename — triggers CPD).
func HandleB(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Service", "bad")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"ok":true,"route":"b"}`)
}

// V6: duplicated block #3.
func HandleC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Service", "bad")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"ok":true,"route":"c"}`)
}

// V7: no tests — entire package coverage = 0%.
