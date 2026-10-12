-- Two-step login with a time-based code from an authenticator app. The
-- secret is sealed with the key file (internal/secret), like calendar
-- addresses. A row with enabled = 0 is a setup that waits for its first
-- code; login only asks for a code once it is 1. last_step is the newest
-- 30-second step whose code was used, so no code works twice.
--
-- Recovery codes are random (80 bits), so only their SHA-256 hash is kept,
-- like API tokens. Using one deletes it. They go when two-step login is
-- turned off, through the cascade.
--
-- Rollback: restore gdash.db.bak-v16, which GDash writes before applying
-- this migration.
CREATE TABLE two_step (
    user_id       INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    secret_sealed TEXT NOT NULL,
    enabled       INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    last_step     INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL
) STRICT;

CREATE TABLE recovery_codes (
    user_id   INTEGER NOT NULL REFERENCES two_step (user_id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    PRIMARY KEY (user_id, code_hash)
) STRICT;
