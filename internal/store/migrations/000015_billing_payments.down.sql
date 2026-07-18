DROP INDEX IF EXISTS subscriptions_processor_subscription_ref;

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS subscription_ref,
    DROP COLUMN IF EXISTS processor;

DROP TABLE IF EXISTS processor_events;
DROP TABLE IF EXISTS billing_customers;
DROP TABLE IF EXISTS plan_prices;
