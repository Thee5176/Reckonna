-- Query-side reads. SELECT ONLY — this package is imported by cmd/query, which
-- must have NO write path (IT9). Every read is scoped by owner_sub so one
-- owner never sees another's rows (AT3, IT5).

-- name: GetJournalEntry :one
SELECT id, entry_date, description, owner_sub, book_id, version, created_at, updated_at
FROM journal_entry
WHERE id = $1 AND owner_sub = $2;

-- name: ListJournalEntries :many
-- Cursor pagination on the time-ordered UUIDv7 PK. A NULL cursor starts at the
-- first page; the handler asks for limit+1 rows to compute has_more.
SELECT id, entry_date, description, owner_sub, book_id, version, created_at, updated_at
FROM journal_entry
WHERE owner_sub = @owner_sub
  AND (sqlc.narg('cursor')::uuid IS NULL OR id > sqlc.narg('cursor')::uuid)
ORDER BY id ASC
LIMIT @page_limit;

-- name: GetJournalLines :many
-- Lines of one entry, owner-scoped via the parent entry.
SELECT jl.id, jl.journal_entry_id, a.code AS account_code, jl.side, jl.amount, jl.line_no
FROM journal_line jl
JOIN account a ON a.id = jl.account_id
JOIN journal_entry je ON je.id = jl.journal_entry_id
WHERE jl.journal_entry_id = @entry_id AND je.owner_sub = @owner_sub
ORDER BY jl.line_no;
