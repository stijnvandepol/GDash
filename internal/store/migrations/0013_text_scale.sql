-- Text size per display, in percent: a small screen such as a Nest Hub
-- shows a dashboard larger, with fewer items, than a TV across the room.
--
-- Rollback: restore gdash.db.bak-v12, which GDash writes before applying
-- this migration.
ALTER TABLE displays ADD COLUMN text_scale INTEGER NOT NULL DEFAULT 100;
