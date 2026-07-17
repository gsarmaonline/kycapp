CREATE TABLE api_keys (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    key_prefix  TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE TABLE audit_events (
    id               TEXT PRIMARY KEY,
    actor            TEXT NOT NULL,
    method           TEXT NOT NULL,
    path             TEXT NOT NULL,
    status_code      INTEGER NOT NULL,
    organisation_id  TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_created_at_idx ON audit_events (created_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events (actor);
