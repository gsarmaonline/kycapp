-- name: UpsertProductPlanPrice :one
INSERT INTO product_plan_prices (
    id, product_plan_id, interval, currency, unit_amount, processor,
    processor_product_ref, processor_price_ref, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (product_plan_id, interval, processor) DO UPDATE
SET currency = EXCLUDED.currency,
    unit_amount = EXCLUDED.unit_amount,
    processor_product_ref = EXCLUDED.processor_product_ref,
    processor_price_ref = EXCLUDED.processor_price_ref,
    status = EXCLUDED.status
RETURNING *;

-- name: GetProductPlanPrice :one
SELECT * FROM product_plan_prices
WHERE id = $1;

-- name: GetProductPlanPriceByPlanInterval :one
SELECT * FROM product_plan_prices
WHERE product_plan_id = $1
  AND interval = $2
  AND processor = $3;

-- name: GetProductPlanPriceByProcessorRef :one
SELECT * FROM product_plan_prices
WHERE processor = $1
  AND processor_price_ref = $2;

-- name: ListProductPlanPricesByPlan :many
SELECT * FROM product_plan_prices
WHERE product_plan_id = $1
ORDER BY interval;

-- name: ListProductPlanPricesByOrg :many
SELECT ppp.*
FROM product_plan_prices ppp
JOIN product_plans pp ON pp.id = ppp.product_plan_id
WHERE pp.organisation_id = $1
ORDER BY pp.key, ppp.interval;

-- name: ListUnsyncedProductPlanPricesByOrg :many
SELECT ppp.*
FROM product_plan_prices ppp
JOIN product_plans pp ON pp.id = ppp.product_plan_id
WHERE pp.organisation_id = $1
  AND ppp.processor = $2
  AND ppp.status = 'active'
  AND (ppp.processor_price_ref = '' OR ppp.processor_product_ref = '')
ORDER BY pp.key, ppp.interval;
