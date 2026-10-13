-- Web pages for the Web page widget: a page that an admin sets up under
-- Integrations, which a TV shows in a sandboxed frame. The address is shown in
-- the web interface and sent to the screens whose widgets show it, so it
-- is not sealed. refresh_minutes 0 never reloads the page; zoom is in
-- percent; allow_scripts 1 lets the page run scripts in its frame.
--
-- Rollback: restore gdash.db.bak-v21, which GDash writes before applying
-- this migration.
CREATE TABLE web_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    refresh_minutes INTEGER NOT NULL CHECK (refresh_minutes BETWEEN 0 AND 1440),
    zoom            INTEGER NOT NULL CHECK (zoom BETWEEN 50 AND 200),
    allow_scripts   INTEGER NOT NULL CHECK (allow_scripts IN (0, 1)),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
) STRICT;
