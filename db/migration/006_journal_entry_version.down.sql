-- 006_journal_entry_version.down.sql — reverse of 006.
DROP TRIGGER IF EXISTS trg_journal_entry_version ON journal_entry;
DROP FUNCTION IF EXISTS bump_journal_entry_version();
ALTER TABLE journal_entry DROP COLUMN IF EXISTS version;
