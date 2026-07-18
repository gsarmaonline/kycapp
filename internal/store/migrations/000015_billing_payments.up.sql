-- Stripe executor: plan prices, billing customers, webhook inbox, subscription processor refs.

CREATE TABLE plan_prices (
    id                   TEXT PRIMARY KEY,
    plan_id              TEXT NOT NULL REFERENCES plans (id) ON DELETE CASCADE,
    interval             TEXT NOT NULL CHECK (interval IN ('month', 'year')),
    currency             TEXT NOT NULL DEFAULT 'usd',
    unit_amount          BIGINT NOT NULL CHECK (unit_amount >= 0),
    processor            TEXT NOT NULL DEFAULT 'stripe',
    processor_price_ref  TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    UNIQUE (plan_id, interval, processor),
    UNIQUE (processor, processor_price_ref)
);

CREATE TABLE billing_customers (
    id                TEXT PRIMARY KEY,
    organisation_id   TEXT NOT NULL UNIQUE REFERENCES organisations (id) ON DELETE CASCADE,
    processor         TEXT NOT NULL DEFAULT 'stripe',
    customer_ref      TEXT NOT NULL,
    email             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (processor, customer_ref)
);

CREATE TABLE processor_events (
    id            TEXT PRIMARY KEY,
    processor     TEXT NOT NULL,
    event_ref     TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    processed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (processor, event_ref)
);

ALTER TABLE subscriptions
    ADD COLUMN processor TEXT,
    ADD COLUMN subscription_ref TEXT;

CREATE UNIQUE INDEX subscriptions_processor_subscription_ref
    ON subscriptions (processor, subscription_ref)
    WHERE subscription_ref IS NOT NULL AND subscription_ref <> '';
