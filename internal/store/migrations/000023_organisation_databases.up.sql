CREATE TABLE organisation_databases (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    driver           TEXT NOT NULL DEFAULT 'postgres'
                     CHECK (driver IN ('postgres')),
    host             TEXT NOT NULL,
    port             INTEGER NOT NULL DEFAULT 5432,
    database_name    TEXT NOT NULL,
    username         TEXT NOT NULL,
    password         TEXT NOT NULL DEFAULT '',
    ssl_mode         TEXT NOT NULL DEFAULT 'require',
    status           TEXT NOT NULL DEFAULT 'connected'
                     CHECK (status IN ('connected', 'disconnected')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX organisation_databases_org_idx
    ON organisation_databases (organisation_id);
