// Package command holds the write-side Gin handlers (POST/PUT/DELETE
// journal-entries) plus their DTO mapping, error classification, and the
// Idempotency-Key middleware. It imports the command service + repository, so it
// must never be imported by cmd/query (compile-time CQRS purity, IT9).
package command

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/thee5176/reckonna/internal/domain"
	"github.com/thee5176/reckonna/internal/service"
)

// errBadRequest marks a syntactic request error (malformed field, bad type) →
// 400. Business-rule violations come from the service/domain and map to 422.
var errBadRequest = errors.New("bad request")

// lineDTO is one posting in the request body.
type lineDTO struct {
	AccountCode int               `json:"account_code"`
	Side        string            `json:"side"`
	Amount      string            `json:"amount"`
	Dimensions  map[string]string `json:"dimensions"`
}

// entryDTO is the POST/PUT request body. Owner is never in the body — it comes
// from the JWT sub. Book defaults to base in v1.
type entryDTO struct {
	Date        string    `json:"date"`
	Description string    `json:"description"`
	Lines       []lineDTO `json:"lines"`
}

// toInput maps the DTO to a service.EntryInput, owned by the authenticated sub.
// Syntactic problems (bad date/side/amount, no lines) return errBadRequest (400);
// semantic validation (balance, currency, dimensions, unknown account) is the
// service's job (422).
func (d entryDTO) toInput(owner string) (service.EntryInput, error) {
	if len(d.Lines) == 0 {
		return service.EntryInput{}, fmt.Errorf("%w: at least one line required", errBadRequest)
	}
	date, err := time.Parse("2006-01-02", d.Date)
	if err != nil {
		return service.EntryInput{}, fmt.Errorf("%w: date must be YYYY-MM-DD", errBadRequest)
	}

	lines := make([]service.LineInput, len(d.Lines))
	for i, l := range d.Lines {
		var side domain.Side
		switch l.Side {
		case "debit":
			side = domain.SideDebit
		case "credit":
			side = domain.SideCredit
		default:
			return service.EntryInput{}, fmt.Errorf("%w: line %d side must be debit|credit", errBadRequest, i)
		}
		amt, err := decimal.NewFromString(l.Amount)
		if err != nil {
			return service.EntryInput{}, fmt.Errorf("%w: line %d amount not a decimal", errBadRequest, i)
		}
		if amt.IsNegative() {
			return service.EntryInput{}, fmt.Errorf("%w: line %d amount must be >= 0", errBadRequest, i)
		}
		dims := make(map[domain.DimensionType]string, len(l.Dimensions))
		for k, v := range l.Dimensions {
			dims[domain.DimensionType(k)] = v
		}
		lines[i] = service.LineInput{
			AccountCode: l.AccountCode,
			Side:        side,
			Amount:      domain.NewMoney(amt),
			Dimensions:  dims,
		}
	}

	return service.EntryInput{
		Date:        date,
		Description: d.Description,
		Owner:       owner,
		Book:        domain.BookBase,
		Lines:       lines,
	}, nil
}
