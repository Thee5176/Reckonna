-- 002_balance_check.down.sql — reverse of 002.
DROP TRIGGER IF EXISTS trg_entry_balanced ON journal_line;
DROP FUNCTION IF EXISTS assert_entry_balanced();
