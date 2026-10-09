-- The theme of a dashboard, one of the built-in themes in internal/display.
--
-- Rollback: restore nookboard.db.bak-v5, which Nookboard writes before applying
-- this migration.
ALTER TABLE dashboards ADD COLUMN theme TEXT NOT NULL DEFAULT 'minimal-dark';
