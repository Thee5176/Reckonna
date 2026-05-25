---
name: tdd-go
description: TDD workflow and Java->Go translation patterns for the CQRS accounting rewrite. Apply when writing Go tests or porting Spring Boot code.
---
# TDD for the Go CQRS Rewrite

## Cycle
RED (failing table-driven test) -> GREEN (minimal code) -> REFACTOR (tests stay green).
The PostToolUse hook enforces fmt/lint/test on every edit.

## Table-driven test template
```go
func TestPostLedger(t *testing.T) {
    tests := []struct {
        name    string
        items   []domain.LedgerItem
        wantErr error
    }{
        {"balanced", balanced(), nil},
        {"unbalanced rejected", unbalanced(), domain.ErrUnbalanced}, // REQUIRED for money paths
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := newTestService(t)                          // Arrange
            err := svc.Post(context.Background(), tt.items)   // Act
            require.ErrorIs(t, err, tt.wantErr)               // Assert
        })
    }
}
```

## Integration tests
Use `testcontainers-go` to spin up Postgres; run golang-migrate, then sqlc-backed repos
against the real DB. CQRS: assert a command-side write becomes visible on the query side.

## Java -> Go mapping
| Spring Boot / Java          | Go equivalent                                  |
|-----------------------------|------------------------------------------------|
| JOOQ codegen (DB-first)     | sqlc (DB-first, same top-down workflow)        |
| Flyway                      | golang-migrate (NNN_*.up/down.sql)             |
| @Transactional multi-step   | pgx.Tx with explicit Begin/Commit/Rollback     |
| ModelMapper DTO<->Entity    | explicit mapping funcs (no reflection)         |
| @RestController             | Gin handler funcs                              |
| @Service + DI               | plain structs, constructor injection (or wire) |
| JUnit + Mockito             | go test + testify; moq/gomock for interfaces   |
| AAA test structure          | same: Arrange / Act / Assert blocks            |
| Optional<T>                 | (T, error) or *T                               |
| checked exceptions          | returned error, wrapped with %w                |

## Invariant helper
```go
func NewLedger(items []LedgerItem) (*Ledger, error) {
    var d, c int64
    for _, it := range items { d += it.Debit; c += it.Credit }
    if d != c { return nil, ErrUnbalanced }
    return &Ledger{Items: items}, nil
}
```
