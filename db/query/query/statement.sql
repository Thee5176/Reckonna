-- Query-side financial statements. SELECT ONLY. Aggregates are owner-scoped and
-- grouped by CoA element (IT7). The service converts signed sums into presented
-- positive balances using each account's normal_balance.

-- name: BalanceSheet :many
-- net_debit = Σ(debit) − Σ(credit) per account. Asset balance = net_debit;
-- liability/equity balance = −net_debit. Assets == liabilities + equity (AT6).
SELECT a.type,
       a.code,
       a.name,
       a.normal_balance,
       COALESCE(SUM(CASE WHEN jl.side = 'debit' THEN jl.amount ELSE -jl.amount END), 0)::numeric AS net_debit
FROM account a
LEFT JOIN journal_line  jl ON jl.account_id = a.id
LEFT JOIN journal_entry je ON je.id = jl.journal_entry_id AND je.owner_sub = @owner_sub
WHERE a.type IN ('asset', 'liability', 'equity')
GROUP BY a.type, a.code, a.name, a.normal_balance
ORDER BY a.code;

-- name: ProfitLoss :many
-- net_credit = Σ(credit) − Σ(debit) per account. Income adds, expense subtracts;
-- netIncome = Σ net_credit over all P&L accounts (AT7).
SELECT a.type,
       a.code,
       a.name,
       COALESCE(SUM(CASE WHEN jl.side = 'credit' THEN jl.amount ELSE -jl.amount END), 0)::numeric AS net_credit
FROM account a
LEFT JOIN journal_line  jl ON jl.account_id = a.id
LEFT JOIN journal_entry je ON je.id = jl.journal_entry_id AND je.owner_sub = @owner_sub
WHERE a.type IN ('income', 'expense')
GROUP BY a.type, a.code, a.name
ORDER BY a.code;
