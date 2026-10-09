-- The theme of a dashboard, one of the built-in themes in internal/display.
--
-- Rollback: restore gdash.db.bak-v5, which GDash writes before applying
-- this migration.
ALTER TABLE dashboards ADD COLUMN theme TEXT NOT NULL DEFAULT 'minimal-dark';
