-- Multiple inbound webhook endpoints per org (replaces single-row organisation_inbound_hooks).
CREATE TABLE organisation_inbound_webhooks (
    id               TEXT PRIMARY KEY,
    organisation_id  TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    secret           TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'disconnected'
                     CHECK (status IN ('connected', 'disconnected')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX organisation_inbound_webhooks_org_idx
    ON organisation_inbound_webhooks (organisation_id);

-- Migrate legacy single inbound hook if present.
INSERT INTO organisation_inbound_webhooks (id, organisation_id, name, secret, status, created_at, updated_at)
SELECT
    organisation_id || '_inbound',
    organisation_id,
    'Default inbound',
    secret,
    status,
    updated_at,
    updated_at
FROM organisation_inbound_hooks
WHERE COALESCE(secret, '') <> '' OR status = 'connected';

DROP TABLE IF EXISTS organisation_inbound_hooks;
