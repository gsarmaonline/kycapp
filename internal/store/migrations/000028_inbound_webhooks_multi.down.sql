-- Reverse multi inbound: restore single-row table from first inbound webhook per org.
CREATE TABLE organisation_inbound_hooks (
    organisation_id TEXT PRIMARY KEY REFERENCES organisations (id) ON DELETE CASCADE,
    secret          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'disconnected'
                    CHECK (status IN ('connected', 'disconnected')),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO organisation_inbound_hooks (organisation_id, secret, status, updated_at)
SELECT DISTINCT ON (organisation_id)
    organisation_id, secret, status, updated_at
FROM organisation_inbound_webhooks
ORDER BY organisation_id, created_at ASC;

DROP TABLE IF EXISTS organisation_inbound_webhooks;
