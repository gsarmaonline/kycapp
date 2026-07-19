-- Org-scoped API keys + integration credentials (Stripe) for Settings.

ALTER TABLE api_keys
    ADD COLUMN organisation_id TEXT REFERENCES organisations (id);

CREATE INDEX api_keys_org_idx ON api_keys (organisation_id)
    WHERE organisation_id IS NOT NULL;

CREATE TABLE organisation_integrations (
    organisation_id  TEXT NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'connected'
                     CHECK (status IN ('connected', 'disconnected')),
    secret_key       TEXT NOT NULL DEFAULT '',
    public_key       TEXT NOT NULL DEFAULT '',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organisation_id, provider)
);
