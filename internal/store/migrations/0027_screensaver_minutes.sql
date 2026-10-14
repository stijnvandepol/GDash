-- How often a Cast display's receiver is restarted to keep the dashboard in
-- front of the TV's screensaver, in minutes; 0 is never. 15 is what every
-- Cast display did before.
--
-- Rollback: restore the gdash.db.bak-v<N> that GDash writes before applying
-- this migration, N being the schema version before it.
ALTER TABLE displays ADD COLUMN screensaver_minutes INTEGER NOT NULL DEFAULT 15;
