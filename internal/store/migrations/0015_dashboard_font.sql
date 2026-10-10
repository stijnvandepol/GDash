-- The typeface of a dashboard, one of the fonts in internal/display; empty
-- means the theme's own.
--
-- Rollback: restore gdash.db.bak-v14, which GDash writes before applying
-- this migration.
ALTER TABLE dashboards ADD COLUMN font TEXT NOT NULL DEFAULT '';
