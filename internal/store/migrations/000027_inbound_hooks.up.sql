-- Inbound automation webhook (org-scoped). One endpoint per org; secret auth.
CREATE TABLE organisation_inbound_hooks (
    organisation_id TEXT PRIMARY KEY REFERENCES organisations (id) ON DELETE CASCADE,
    secret          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'disconnected'
                    CHECK (status IN ('connected', 'disconnected')),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
