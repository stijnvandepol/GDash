-- Accounts for the web interface. Sessions store only a SHA-256 hash of the
-- session token, so a leaked database backup cannot be used to log in.
--
-- Rollback: restore gdash.db.bak-v1, which GDash writes before applying
-- this migration.
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL
) STRICT;

CREATE TABLE sessions (
    token_hash   TEXT PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
) STRICT;

CREATE INDEX sessions_by_last_seen ON sessions (last_seen_at);
