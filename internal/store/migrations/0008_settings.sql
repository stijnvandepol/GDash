-- Data sources managed in the web interface instead of environment
-- variables. settings holds single values as JSON by key, such as the
-- weather location. sources holds news feeds and calendars; a calendar's
-- address is a secret and is stored encrypted by internal/secret.
--
-- Rollback: restore nookboard.db.bak-v7, which Nookboard writes before applying
-- this migration.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE TABLE sources (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    kind       TEXT NOT NULL CHECK (kind IN ('news', 'calendar')),
    name       TEXT NOT NULL,
    address    TEXT NOT NULL,
    host       TEXT NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

CREATE INDEX sources_kind ON sources (kind, id);
