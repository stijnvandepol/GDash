-- Displays are physical screens. The name is what the user sees; the id is
-- the stable slug used in /d/<id>. cast_device_id links a display to a Cast
-- device by its device ID, not its IP address, because DHCP can change the
-- address.
--
-- Rollback: Nookboard copies the database to nookboard.db.bak-v<old version>
-- before applying a migration; restore that file to undo.
CREATE TABLE displays (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    cast_device_id TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
) STRICT;
