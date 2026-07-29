-- Observability database: semantic activity + usage meters (separate from primary KYC DB).

CREATE TABLE activity_events (
    id                  TEXT PRIMARY KEY,
    organisation_id     TEXT NOT NULL,
    organisation_slug   TEXT NOT NULL DEFAULT '',
    organisation_name   TEXT NOT NULL DEFAULT '',
    actor_type          TEXT NOT NULL DEFAULT '',
    actor_id            TEXT NOT NULL DEFAULT '',
    actor_label         TEXT NOT NULL DEFAULT '',
    action              TEXT NOT NULL,
    resource_type       TEXT NOT NULL DEFAULT '',
    resource_id         TEXT NOT NULL DEFAULT '',
    summary             TEXT NOT NULL DEFAULT '',
    payload             JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX activity_events_org_created_idx
    ON activity_events (organisation_id, created_at DESC);

CREATE INDEX activity_events_action_created_idx
    ON activity_events (action, created_at DESC);

-- Period rollups for hot-path meters (entitlement checks, etc.).
CREATE TABLE usage_counters (
    organisation_id  TEXT NOT NULL,
    meter_key        TEXT NOT NULL,
    period_start     TIMESTAMPTZ NOT NULL,
    dim1_key         TEXT NOT NULL DEFAULT '',
    dim1_value       TEXT NOT NULL DEFAULT '',
    dim2_key         TEXT NOT NULL DEFAULT '',
    dim2_value       TEXT NOT NULL DEFAULT '',
    count            BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (
        organisation_id,
        meter_key,
        period_start,
        dim1_key,
        dim1_value,
        dim2_key,
        dim2_value
    )
);

CREATE INDEX usage_counters_org_period_idx
    ON usage_counters (organisation_id, period_start DESC);
