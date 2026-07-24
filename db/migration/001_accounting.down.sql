-- 001_accounting.down.sql — reverse of 001 (drop in FK-dependency order).
DROP TABLE IF EXISTS journal_line_dimension;
DROP TABLE IF EXISTS journal_line;
DROP TABLE IF EXISTS journal_entry;
DROP TABLE IF EXISTS dimension_value;
DROP TABLE IF EXISTS dimension_type;
DROP TABLE IF EXISTS book;
DROP TABLE IF EXISTS account;

DROP TYPE IF EXISTS entry_side;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS current_noncurrent;
DROP TYPE IF EXISTS normal_balance;
DROP TYPE IF EXISTS account_type;
