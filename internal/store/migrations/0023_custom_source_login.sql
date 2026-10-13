-- A custom source can log in first: auth 'login' posts a password to a
-- login address on the same host and sends the session token it gets back
-- with every request, as Pi-hole v6 wants. login holds how, as JSON
-- {url, body, tokenPath, place, validityPath} checked by internal/custom,
-- or empty for the other kinds; the body is a template with {secret} where
-- the password goes, never the password itself, which stays sealed in
-- secrets_sealed. The session token is kept in memory only.
--
-- SQLite cannot change a CHECK constraint, so the table is rebuilt with the
-- same columns and rows. Its AUTOINCREMENT counter is carried over, so the
-- ID of a deleted source, which a widget may still name, is never reused.
--
-- Rollback: restore gdash.db.bak-v22, which GDash writes before applying
-- this migration.
CREATE TABLE custom_sources_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    method           TEXT NOT NULL CHECK (method IN ('GET', 'POST')),
    body             TEXT NOT NULL DEFAULT '',
    auth             TEXT NOT NULL CHECK (auth IN ('none', 'bearer', 'header', 'query', 'basic', 'login')),
    auth_name        TEXT NOT NULL DEFAULT '',
    headers          TEXT NOT NULL DEFAULT '[]',
    secrets_sealed   TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL CHECK (interval_seconds BETWEEN 30 AND 86400),
    sample           TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    named_values     TEXT NOT NULL DEFAULT '',
    mode             TEXT NOT NULL DEFAULT 'fetch' CHECK (mode IN ('fetch', 'push')),
    login            TEXT NOT NULL DEFAULT ''
) STRICT;

INSERT INTO custom_sources_new (id, name, url, method, body, auth, auth_name, headers, secrets_sealed, interval_seconds,
                                sample, created_at, updated_at, named_values, mode)
SELECT id, name, url, method, body, auth, auth_name, headers, secrets_sealed, interval_seconds,
       sample, created_at, updated_at, named_values, mode
FROM custom_sources;

DELETE FROM sqlite_sequence WHERE name = 'custom_sources_new';
INSERT INTO sqlite_sequence (name, seq) SELECT 'custom_sources_new', seq FROM sqlite_sequence WHERE name = 'custom_sources';

DROP TABLE custom_sources;
ALTER TABLE custom_sources_new RENAME TO custom_sources;
