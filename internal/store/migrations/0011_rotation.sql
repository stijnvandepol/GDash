-- Page rotation: a display can show other dashboards in turn with its own,
-- each for rotate_seconds. The screen does the turning; the server sends all
-- pages. Deleting a dashboard removes it from every rotation.
--
-- Rollback: restore gdash.db.bak-v10, which GDash writes before applying
-- this migration.
ALTER TABLE displays ADD COLUMN rotate_seconds INTEGER NOT NULL DEFAULT 0;

CREATE TABLE display_rotation (
    display_id   TEXT NOT NULL REFERENCES displays (id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    dashboard_id TEXT NOT NULL REFERENCES dashboards (id) ON DELETE CASCADE,
    PRIMARY KEY (display_id, position)
) STRICT;

CREATE INDEX display_rotation_dashboard ON display_rotation (dashboard_id);
