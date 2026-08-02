CREATE TABLE remote_access_config (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    enabled     INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    listen_port INTEGER NOT NULL DEFAULT 51820 CHECK (listen_port BETWEEN 1 AND 65535),
    endpoint    TEXT NOT NULL DEFAULT '',
    updated_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

INSERT INTO remote_access_config (id) VALUES (1);

CREATE TABLE remote_access_peers (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 64),
    public_key  TEXT NOT NULL UNIQUE,
    address     TEXT NOT NULL,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    revoked_at  INTEGER
);

CREATE UNIQUE INDEX remote_access_active_address
    ON remote_access_peers(address)
    WHERE revoked_at IS NULL;

CREATE INDEX remote_access_active_peers
    ON remote_access_peers(revoked_at, id);
