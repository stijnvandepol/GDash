-- Wrong two-step codes in a row per account, and when the last one came.
-- They make the next code wait and, at 50, refuse codes from the app until
-- a recovery code or `gdash reset-password` (internal/auth/codelimit.go).
-- They are stored, not kept in memory, so a restart lifts neither the wait
-- nor that stop. Only a code that is checked counts, and codes are only
-- checked after the right password, so nobody without it can add to them.
-- last_failed_at is empty while failed_codes is 0.
--
-- Rollback: restore gdash.db.bak-v18, which GDash writes before applying
-- this migration.
ALTER TABLE two_step ADD COLUMN failed_codes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE two_step ADD COLUMN last_failed_at TEXT NOT NULL DEFAULT '';
