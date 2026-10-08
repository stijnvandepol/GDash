-- Active hours per display, as HH:MM in the server's time zone. Both empty
-- means always on. Outside them Daycast stops casting and the page goes dark.
--
-- Rollback: restore daycast.db.bak-v4, which Daycast writes before applying
-- this migration.
ALTER TABLE displays ADD COLUMN active_from TEXT NOT NULL DEFAULT '';
ALTER TABLE displays ADD COLUMN active_until TEXT NOT NULL DEFAULT '';
