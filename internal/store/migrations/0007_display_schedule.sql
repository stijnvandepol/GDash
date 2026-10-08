-- A day plan per display: during each slot the display shows another
-- dashboard than its own, such as a morning dashboard from 06:30 to 09:00.
-- The first slot that contains the current time wins. Deleting a display
-- or a dashboard removes its slots.
--
-- Rollback: restore daycast.db.bak-v6, which Daycast writes before applying
-- this migration.
CREATE TABLE display_schedule (
    display_id   TEXT NOT NULL REFERENCES displays (id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    starts       TEXT NOT NULL,
    ends         TEXT NOT NULL,
    dashboard_id TEXT NOT NULL REFERENCES dashboards (id) ON DELETE CASCADE,
    PRIMARY KEY (display_id, position)
) STRICT;

CREATE INDEX display_schedule_dashboard ON display_schedule (dashboard_id);
