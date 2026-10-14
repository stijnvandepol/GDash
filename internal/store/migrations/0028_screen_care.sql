-- What a display does for its screen: 'oled' care against burn-in, and
-- when it is dimmed: 'sunset' between sunset and sunrise at the weather
-- location, or 'hours' from dim_from to dim_until (HH:MM). Every display
-- starts with both off, as it was before.
--
-- Rollback: restore gdash.db.bak-v27, which GDash writes before applying
-- this migration.
ALTER TABLE displays ADD COLUMN screen_care TEXT NOT NULL DEFAULT 'off';
ALTER TABLE displays ADD COLUMN dim_at_night TEXT NOT NULL DEFAULT 'off';
ALTER TABLE displays ADD COLUMN dim_from TEXT NOT NULL DEFAULT '';
ALTER TABLE displays ADD COLUMN dim_until TEXT NOT NULL DEFAULT '';
