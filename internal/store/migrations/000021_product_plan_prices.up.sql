-- Merchant product plan prices (org Stripe catalog sync).

CREATE TABLE product_plan_prices (
    id                     TEXT PRIMARY KEY,
    product_plan_id        TEXT NOT NULL REFERENCES product_plans (id) ON DELETE CASCADE,
    interval               TEXT NOT NULL CHECK (interval IN ('month', 'year')),
    currency               TEXT NOT NULL DEFAULT 'usd',
    unit_amount            BIGINT NOT NULL CHECK (unit_amount >= 0),
    processor              TEXT NOT NULL DEFAULT 'stripe',
    processor_product_ref  TEXT NOT NULL DEFAULT '',
    processor_price_ref    TEXT NOT NULL DEFAULT '',
    status                 TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    UNIQUE (product_plan_id, interval, processor)
);

CREATE UNIQUE INDEX product_plan_prices_processor_price_ref
    ON product_plan_prices (processor, processor_price_ref)
    WHERE processor_price_ref <> '';
