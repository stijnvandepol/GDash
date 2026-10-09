-- Security-relevant events: logins, password changes, pairing and changes
-- to displays, dashboards and settings. Kept for 90 days and at most 5000
-- events (internal/store/audit.go). A failed login stores no username,
-- because people sometimes type their password into that field.
--
-- Rollback: restore gdash.db.bak-v9, which GDash writes before applying
-- this migration.
CREATE TABLE audit_log (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    at     TEXT NOT NULL,
    actor  TEXT NOT NULL,
    action TEXT NOT NULL,
    detail TEXT NOT NULL,
    ip     TEXT NOT NULL
) STRICT;

CREATE INDEX audit_log_at ON audit_log (at);
