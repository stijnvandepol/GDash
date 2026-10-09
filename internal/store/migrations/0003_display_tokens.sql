-- Paired screens. A screen proves it may show a display with a token in a
-- cookie; only its SHA-256 hash is stored. Deleting a display unpairs every
-- screen showing it. kind is 'screen' for a screen paired with a code and
-- 'cast' for the page a cast keeper opens; a display has at most one 'cast'
-- token, replaced on every launch.
--
-- Rollback: restore nookboard.db.bak-v2, which Nookboard writes before applying
-- this migration.
CREATE TABLE display_tokens (
    token_hash   TEXT PRIMARY KEY,
    display_id   TEXT NOT NULL REFERENCES displays (id) ON DELETE CASCADE,
    kind         TEXT NOT NULL CHECK (kind IN ('screen', 'cast')),
    created_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
) STRICT;

CREATE INDEX display_tokens_by_display ON display_tokens (display_id);
