-- Custom API sources: any JSON HTTP API that an admin sets up in the web
-- interface, for the Custom widget. The address, method, body, the name of
-- the authentication (a header or query parameter name, or the basic auth
-- user name) and the extra headers are shown in the web interface. Every
-- secret value (a bearer token, an API key, a basic auth password and the
-- values of headers marked secret) is sealed together in secrets_sealed
-- with the key file (internal/secret), like calendar addresses; headers
-- holds the secret headers with an empty value. sample is a JSON answer the
-- admin pasted to design a widget against, or empty.
--
-- Rollback: restore gdash.db.bak-v17, which GDash writes before applying
-- this migration.
CREATE TABLE custom_sources (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    method           TEXT NOT NULL CHECK (method IN ('GET', 'POST')),
    body             TEXT NOT NULL DEFAULT '',
    auth             TEXT NOT NULL CHECK (auth IN ('none', 'bearer', 'header', 'query', 'basic')),
    auth_name        TEXT NOT NULL DEFAULT '',
    headers          TEXT NOT NULL DEFAULT '[]',
    secrets_sealed   TEXT NOT NULL DEFAULT '',
    interval_seconds INTEGER NOT NULL CHECK (interval_seconds BETWEEN 30 AND 86400),
    sample           TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
) STRICT;
