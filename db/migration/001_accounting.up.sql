-- 001_accounting.up.sql — core accounting schema (plan 03 S3).
-- Naming per CoA governance standard §7: journal_entry (header) + journal_line
-- (postings); account · book · dimension_type · dimension_value ·
-- journal_line_dimension. UUIDv7 PKs are supplied by the Go app (google/uuid);
-- reference rows use gen_random_uuid(). Money is NUMERIC(20,4) — never float.

CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- gen_random_uuid() for reference rows

-- ── Enumerated domains (mirror internal/domain constants) ──
CREATE TYPE account_type        AS ENUM ('asset', 'liability', 'equity', 'income', 'expense');
CREATE TYPE normal_balance      AS ENUM ('debit', 'credit');
CREATE TYPE current_noncurrent  AS ENUM ('current', 'non_current');
CREATE TYPE account_status      AS ENUM ('active', 'inactive', 'archived');
CREATE TYPE entry_side          AS ENUM ('debit', 'credit');

-- ── account — chart of accounts (standard §6). Currency-neutral (P1). ──
CREATE TABLE account (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code                int  NOT NULL UNIQUE CHECK (code BETWEEN 10000 AND 99999),
    name                text NOT NULL,
    description         text NOT NULL,
    type                account_type   NOT NULL,
    normal_balance      normal_balance NOT NULL,
    postable            boolean NOT NULL DEFAULT true,
    current_noncurrent  current_noncurrent,               -- required for asset/liability, null otherwise
    ifrs_line_item      text NOT NULL,
    allowed_books       text[] NOT NULL DEFAULT '{base}',
    required_dimensions text[] NOT NULL DEFAULT '{}',
    status              account_status NOT NULL DEFAULT 'active',
    owner               text NOT NULL,
    -- type ↔ normal_balance consistency (standard §4 ranges): asset/expense post
    -- debit-normal; liability/equity/income post credit-normal.
    CONSTRAINT account_type_balance_consistent CHECK (
        (type IN ('asset', 'expense')            AND normal_balance = 'debit') OR
        (type IN ('liability', 'equity', 'income') AND normal_balance = 'credit')
    )
);

-- ── book — framework treatment layer (standard §8). v1 = base only. ──
CREATE TABLE book (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL UNIQUE,
    name text NOT NULL
);

-- ── dimension_type + dimension_value — context axes (standard §7). ──
CREATE TABLE dimension_type (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL UNIQUE,   -- entity | currency | counterparty
    name text NOT NULL
);

CREATE TABLE dimension_value (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dimension_type_id uuid NOT NULL REFERENCES dimension_type (id) ON DELETE RESTRICT,
    code              text NOT NULL,   -- e.g. JPY, USD for currency
    name              text NOT NULL,
    UNIQUE (dimension_type_id, code)
);

-- ── journal_entry — header. owner_sub is the JWT sub (owner scoping). ──
CREATE TABLE journal_entry (
    id          uuid PRIMARY KEY,                 -- UUIDv7 supplied by app
    entry_date  date NOT NULL,
    description text NOT NULL DEFAULT '',
    owner_sub   text NOT NULL,
    book_id     uuid NOT NULL REFERENCES book (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX journal_entry_owner_idx ON journal_entry (owner_sub, id);

-- ── journal_line — postings. amount is unsigned; side carries direction. ──
CREATE TABLE journal_line (
    id               uuid PRIMARY KEY,            -- UUIDv7 supplied by app
    journal_entry_id uuid NOT NULL REFERENCES journal_entry (id) ON DELETE CASCADE,
    account_id       uuid NOT NULL REFERENCES account (id) ON DELETE RESTRICT,
    side             entry_side NOT NULL,
    amount           numeric(20, 4) NOT NULL CHECK (amount >= 0),
    line_no          int NOT NULL,
    UNIQUE (journal_entry_id, line_no)
);
CREATE INDEX journal_line_entry_idx   ON journal_line (journal_entry_id);
CREATE INDEX journal_line_account_idx ON journal_line (account_id);

-- ── journal_line_dimension — dimension values per line (incl. currency). ──
CREATE TABLE journal_line_dimension (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_line_id    uuid NOT NULL REFERENCES journal_line (id) ON DELETE CASCADE,
    dimension_type_id  uuid NOT NULL REFERENCES dimension_type (id)  ON DELETE RESTRICT,
    dimension_value_id uuid NOT NULL REFERENCES dimension_value (id) ON DELETE RESTRICT,
    UNIQUE (journal_line_id, dimension_type_id)
);
CREATE INDEX journal_line_dimension_line_idx ON journal_line_dimension (journal_line_id);
