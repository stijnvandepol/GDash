-- When a screen is on: any number of blocks per display, each with days of
-- the week (empty means every day) and two times as HH:MM. The screen is on
-- when one block holds the current time; no blocks means always on.
-- Deleting a display removes its blocks. A display's single daily window
-- (active_from, active_until) becomes one block on every day, so every
-- screen behaves as before. The two old columns are no longer read.
--
-- Rollback: restore gdash.db.bak-v28, which GDash writes before applying
-- this migration.
CREATE TABLE display_active (
    display_id TEXT NOT NULL REFERENCES displays (id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    starts     TEXT NOT NULL,
    ends       TEXT NOT NULL,
    days       TEXT NOT NULL,
    PRIMARY KEY (display_id, position)
) STRICT;

INSERT INTO display_active (display_id, position, starts, ends, days)
SELECT id, 0, active_from, active_until, '' FROM displays
WHERE active_from <> '' OR active_until <> '';
