-- Which preset a custom source was made from, such as homewizard-p1, or
-- empty for the household's own API. The presets live in the web interface
-- (web/src/admin/presets); the server only keeps the id, so the
-- Integrations page shows the source under the preset's own card and the
-- dashboard editor offers it as a widget of its own. internal/custom checks
-- the form of the id. Sources from before keep an empty preset.
--
-- Rollback: restore gdash.db.bak-v23, which GDash writes before applying
-- this migration.
ALTER TABLE custom_sources ADD COLUMN preset TEXT NOT NULL DEFAULT '';
