-- Command-side writes for journal entries. WRITE ONLY — never SELECT-for-serve
-- (that lives in the query package). Reference lookups here resolve business
-- keys (account code, book/dimension codes) to ids during a write tx.

-- name: GetBookByCode :one
SELECT id, code, name FROM book WHERE code = $1;

-- name: GetAccountByCode :one
SELECT id, code, name, type, normal_balance, postable, required_dimensions
FROM account WHERE code = $1;

-- name: GetDimensionValue :one
SELECT dv.id, dv.code, dt.code AS type_code
FROM dimension_value dv
JOIN dimension_type dt ON dt.id = dv.dimension_type_id
WHERE dt.code = $1 AND dv.code = $2;

-- name: InsertJournalEntry :one
INSERT INTO journal_entry (id, entry_date, description, owner_sub, book_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, version;

-- name: InsertJournalLine :exec
INSERT INTO journal_line (id, journal_entry_id, account_id, side, amount, line_no)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertJournalLineDimension :exec
INSERT INTO journal_line_dimension (journal_line_id, dimension_type_id, dimension_value_id)
SELECT $1, dt.id, dv.id
FROM dimension_type dt
JOIN dimension_value dv ON dv.dimension_type_id = dt.id
WHERE dt.code = $2 AND dv.code = $3;

-- name: GetJournalEntryForUpdate :one
SELECT id, owner_sub, version FROM journal_entry WHERE id = $1;

-- name: UpdateJournalEntryHeader :one
-- Optimistic concurrency: only updates when the caller's version matches.
-- The BEFORE UPDATE trigger bumps version + updated_at; RETURNING gives the new
-- version. Zero rows affected ⇒ version conflict (handler → 409).
UPDATE journal_entry
SET entry_date = $2, description = $3
WHERE id = $1 AND version = $4
RETURNING version;

-- name: DeleteJournalLinesByEntry :exec
DELETE FROM journal_line WHERE journal_entry_id = $1;

-- name: DeleteJournalEntry :exec
-- Cascade drops journal_line + journal_line_dimension (AT5).
DELETE FROM journal_entry WHERE id = $1 AND owner_sub = $2;
