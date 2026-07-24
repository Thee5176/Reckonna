-- 006_journal_entry_version.up.sql — optimistic concurrency (plan 03 S9a).
-- version is a monotonic int surfaced as the ETag on GET and required via
-- If-Match on PUT (see API Conventions). A BEFORE UPDATE trigger bumps it so
-- the application can never forget to, and also refreshes updated_at.

ALTER TABLE journal_entry ADD COLUMN version int NOT NULL DEFAULT 1;

CREATE OR REPLACE FUNCTION bump_journal_entry_version() RETURNS trigger AS $$
BEGIN
    NEW.version    := OLD.version + 1;
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_journal_entry_version
    BEFORE UPDATE ON journal_entry
    FOR EACH ROW
    EXECUTE FUNCTION bump_journal_entry_version();
