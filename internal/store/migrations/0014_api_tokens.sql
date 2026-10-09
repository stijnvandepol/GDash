-- API tokens for automations, such as a Home Assistant rest_command that
-- posts an announcement. Only the SHA-256 hash of a token is stored; the
-- token itself is shown once, when it is made. last_used_at is empty until
-- the token is used. A token is revoked by deleting its row.
--
-- Rollback: restore nookboard.db.bak-v13, which Nookboard writes before applying
-- this migration.
CREATE TABLE api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,
    last_used_at TEXT NOT NULL DEFAULT ''
) STRICT;
