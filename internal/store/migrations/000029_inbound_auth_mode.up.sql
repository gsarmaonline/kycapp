ALTER TABLE organisation_inbound_webhooks
    ADD COLUMN auth_mode TEXT NOT NULL DEFAULT 'header'
    CHECK (auth_mode IN ('header', 'query', 'path'));
