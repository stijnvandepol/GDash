-- Named values of a custom source: the values and lists an admin picked
-- out of its answer under Settings and gave a name, so the dashboard
-- editor offers them by name. JSON {"values": [{name, path, unit?,
-- decimals?}], "lists": [{name, path, columns: [{name, path}]}]}, checked
-- by internal/custom, or empty for none. A widget stores the paths it
-- shows itself, so nothing else depends on this column.
--
-- Rollback: restore gdash.db.bak-v19, which GDash writes before applying
-- this migration.
ALTER TABLE custom_sources ADD COLUMN named_values TEXT NOT NULL DEFAULT '';
