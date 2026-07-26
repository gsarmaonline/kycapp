-- Org-scoped onboarding UI state (dismiss only; step completion is derived).

CREATE TABLE organisation_onboarding (
    organisation_id TEXT PRIMARY KEY REFERENCES organisations (id) ON DELETE CASCADE,
    dismissed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
