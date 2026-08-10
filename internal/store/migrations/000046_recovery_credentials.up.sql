-- Recovery credentials replace the shared environment token for everything
-- except a genuinely broken database.
--
-- They are ordinary data: minted by a named staff member, with a reason, an
-- expiry, and revocation. Resolving one produces a global-scope grant that the
-- evaluator handles like any other, so there is no bypass in the access path.
CREATE TABLE IF NOT EXISTS recovery_credentials (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    -- Who minted it and why. Both required: a recovery credential with no
    -- stated reason is indistinguishable from a back door.
    granted_by   TEXT NOT NULL REFERENCES users (id),
    reason       TEXT NOT NULL,
    -- Required, unlike an API key. A recovery credential that never expires is
    -- a permanent bypass wearing a different name.
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recovery_credentials_live_idx
    ON recovery_credentials (expires_at) WHERE revoked_at IS NULL;
