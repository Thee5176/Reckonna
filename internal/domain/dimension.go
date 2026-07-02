package domain

// DimensionType is a context axis attached to a journal line, per the CoA
// governance standard §7 (context lives in dimensions, never in account codes).
// v1 types: entity, currency, counterparty. Members are data, added without a
// CoA change.
type DimensionType string

const (
	DimEntity       DimensionType = "entity"
	DimCurrency     DimensionType = "currency"
	DimCounterparty DimensionType = "counterparty"
)

// Valid reports whether t is one of the v1 dimension types.
func (t DimensionType) Valid() bool {
	switch t {
	case DimEntity, DimCurrency, DimCounterparty:
		return true
	default:
		return false
	}
}

// BookBase is the single framework-neutral book in v1 (standard §8, R8.1).
// Delta books (ifrs/gaap) arrive with the measurement phase.
const BookBase = "base"
