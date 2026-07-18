-- name: UpsertPlanPrice :one
INSERT INTO plan_prices (
    id, plan_id, interval, currency, unit_amount, processor, processor_price_ref, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (plan_id, interval, processor) DO UPDATE
SET currency = EXCLUDED.currency,
    unit_amount = EXCLUDED.unit_amount,
    processor_price_ref = EXCLUDED.processor_price_ref,
    status = EXCLUDED.status
RETURNING *;

-- name: GetPlanPrice :one
SELECT * FROM plan_prices
WHERE id = $1;

-- name: GetActivePlanPrice :one
SELECT * FROM plan_prices
WHERE plan_id = $1
  AND interval = $2
  AND processor = $3
  AND status = 'active';

-- name: GetPlanPriceByProcessorRef :one
SELECT * FROM plan_prices
WHERE processor = $1
  AND processor_price_ref = $2;

-- name: ListPlanPricesByPlan :many
SELECT * FROM plan_prices
WHERE plan_id = $1
ORDER BY interval;

-- name: UpsertBillingCustomer :one
INSERT INTO billing_customers (
    id, organisation_id, processor, customer_ref, email, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, now(), now()
)
ON CONFLICT (organisation_id) DO UPDATE
SET processor = EXCLUDED.processor,
    customer_ref = EXCLUDED.customer_ref,
    email = EXCLUDED.email,
    updated_at = now()
RETURNING *;

-- name: GetBillingCustomerByOrganisation :one
SELECT * FROM billing_customers
WHERE organisation_id = $1;

-- name: GetBillingCustomerByProcessorRef :one
SELECT * FROM billing_customers
WHERE processor = $1
  AND customer_ref = $2;

-- name: InsertProcessorEvent :one
INSERT INTO processor_events (id, processor, event_ref, event_type, payload, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (processor, event_ref) DO NOTHING
RETURNING *;

-- name: GetProcessorEvent :one
SELECT * FROM processor_events
WHERE processor = $1
  AND event_ref = $2;

-- name: MarkProcessorEventProcessed :exec
UPDATE processor_events
SET processed_at = now()
WHERE processor = $1
  AND event_ref = $2
  AND processed_at IS NULL;

-- name: UpsertSubscriptionFromProcessor :one
INSERT INTO subscriptions (
    id, organisation_id, plan_id, status, current_period_end, processor, subscription_ref
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (organisation_id) DO UPDATE
SET plan_id = EXCLUDED.plan_id,
    status = EXCLUDED.status,
    current_period_end = EXCLUDED.current_period_end,
    processor = EXCLUDED.processor,
    subscription_ref = EXCLUDED.subscription_ref
RETURNING *;

-- name: GetSubscriptionByProcessorRef :one
SELECT * FROM subscriptions
WHERE processor = $1
  AND subscription_ref = $2;
