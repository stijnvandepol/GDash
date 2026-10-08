-- Days of the week for a time slot, comma-separated (mon,tue,...,sun).
-- Empty means every day, so existing slots keep their meaning.
--
-- Rollback: restore daycast.db.bak-v8, which Daycast writes before applying
-- this migration.
ALTER TABLE display_schedule ADD COLUMN days TEXT NOT NULL DEFAULT '';
