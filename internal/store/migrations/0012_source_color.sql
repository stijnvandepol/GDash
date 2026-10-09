-- A colour per calendar, so the TV shows whose appointment it is. Empty
-- means the colour follows from the calendar's place in the list
-- (internal/settings).
--
-- Rollback: restore daycast.db.bak-v11, which Daycast writes before applying
-- this migration.
ALTER TABLE sources ADD COLUMN color TEXT NOT NULL DEFAULT '';
