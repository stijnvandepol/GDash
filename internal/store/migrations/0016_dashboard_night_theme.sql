-- The theme a dashboard switches to between sunset and sunrise at the
-- weather location, one of the themes in internal/display; empty means it
-- keeps its theme.
--
-- Rollback: restore gdash.db.bak-v15, which GDash writes before applying
-- this migration.
ALTER TABLE dashboards ADD COLUMN night_theme TEXT NOT NULL DEFAULT '';
