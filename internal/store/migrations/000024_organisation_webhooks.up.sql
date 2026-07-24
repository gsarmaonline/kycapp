CREATE TABLE organisation_webhooks (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    secret           TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'connected'
                     CHECK (status IN ('connected', 'disconnected')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX organisation_webhooks_org_idx
    ON organisation_webhooks (organisation_id);
