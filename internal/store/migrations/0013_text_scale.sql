-- Text size per display, in percent: a small screen such as a Nest Hub
-- shows a dashboard larger, with fewer items, than a TV across the room.
--
-- Rollback: restore nookboard.db.bak-v12, which Nookboard writes before applying
-- this migration.
ALTER TABLE displays ADD COLUMN text_scale INTEGER NOT NULL DEFAULT 100;
