-- Query-side chart-of-accounts + outstanding balances. SELECT ONLY.

-- name: ListAccounts :many
SELECT code, name, description, type, normal_balance, postable, current_noncurrent, ifrs_line_item, status
FROM account
WHERE status = 'active'
ORDER BY code;

-- name: GetOutstandingBalances :many
-- Debit/credit totals per requested account, scoped to the caller's entries.
-- LEFT JOINs keep requested accounts that have no postings (zero totals).
SELECT a.code AS account_code,
       a.name AS account_name,
       a.normal_balance,
       COALESCE(SUM(jl.amount) FILTER (WHERE jl.side = 'debit'),  0)::numeric  AS total_debit,
       COALESCE(SUM(jl.amount) FILTER (WHERE jl.side = 'credit'), 0)::numeric  AS total_credit
FROM account a
LEFT JOIN journal_line  jl ON jl.account_id = a.id
LEFT JOIN journal_entry je ON je.id = jl.journal_entry_id AND je.owner_sub = @owner_sub
WHERE a.code = ANY(@codes::int[])
GROUP BY a.code, a.name, a.normal_balance
ORDER BY a.code;
