-- 002_balance_check.up.sql — deferred double-entry invariant at the DB layer
-- (plan 03 S4). Defense in depth: even if the Go domain check is bypassed, an
-- unbalanced entry cannot commit. Deviation from source (source had NO db check).
--
-- DEFERRABLE INITIALLY DEFERRED so a multi-line insert in one tx is validated
-- once, at COMMIT — not per row (a half-inserted entry is transiently
-- unbalanced and that is fine mid-transaction). Fires per (journal_entry_id):
-- an entry carries a single book (§8 R8.4), so per-entry == per-(entry,book).

CREATE OR REPLACE FUNCTION assert_entry_balanced() RETURNS trigger AS $$
DECLARE
    v_entry  uuid;
    v_debit  numeric(20, 4);
    v_credit numeric(20, 4);
BEGIN
    v_entry := COALESCE(NEW.journal_entry_id, OLD.journal_entry_id);

    SELECT COALESCE(SUM(amount) FILTER (WHERE side = 'debit'),  0),
           COALESCE(SUM(amount) FILTER (WHERE side = 'credit'), 0)
      INTO v_debit, v_credit
      FROM journal_line
     WHERE journal_entry_id = v_entry;

    -- A fully-deleted entry sums to (0, 0) and is trivially balanced, so a
    -- cascade DELETE of the whole entry passes.
    IF v_debit <> v_credit THEN
        RAISE EXCEPTION
            'unbalanced journal entry %: debit % <> credit %', v_entry, v_debit, v_credit
            USING ERRCODE = '23514';  -- check_violation → handler maps to 422
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_entry_balanced
    AFTER INSERT OR UPDATE OR DELETE ON journal_line
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION assert_entry_balanced();
