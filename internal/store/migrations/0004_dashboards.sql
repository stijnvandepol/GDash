-- Dashboards made in the editor. The layout is the widget list as JSON,
-- validated by internal/display before it is stored. A display without a
-- dashboard_id shows the "default" dashboard, which GDash creates at
-- start-up; deleting a dashboard sends its displays back to the default.
--
-- Rollback: restore gdash.db.bak-v3, which GDash writes before applying
-- this migration.
CREATE TABLE dashboards (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    widgets    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

ALTER TABLE displays ADD COLUMN dashboard_id TEXT REFERENCES dashboards (id) ON DELETE SET NULL;
