-- name: CreateProductFeature :one
INSERT INTO entitlements (id, key, description, scope, organisation_id)
VALUES ($1, $2, $3, 'product', $4)
RETURNING *;

-- name: GetEntitlement :one
SELECT * FROM entitlements
WHERE id = $1;

-- name: ListProductFeaturesByOrg :many
SELECT * FROM entitlements
WHERE organisation_id = $1
  AND scope = 'product'
ORDER BY key;

-- name: UpdateProductFeature :one
UPDATE entitlements
SET description = $2
WHERE id = $1
  AND organisation_id = $3
  AND scope = 'product'
RETURNING *;

-- name: DeleteProductFeature :exec
DELETE FROM entitlements
WHERE id = $1
  AND organisation_id = $2
  AND scope = 'product';

-- name: ListEntitlementIDsByKeysForOrg :many
SELECT DISTINCT ON (key) id, key
FROM entitlements
WHERE key = ANY(sqlc.arg('keys')::text[])
  AND (organisation_id IS NULL OR organisation_id = sqlc.arg('organisation_id'))
ORDER BY key, (organisation_id IS NOT NULL) DESC;

-- name: ListEntitlementScopesByKeysForOrg :many
SELECT DISTINCT ON (key) key, scope
FROM entitlements
WHERE key = ANY(sqlc.arg('keys')::text[])
  AND (organisation_id IS NULL OR organisation_id = sqlc.arg('organisation_id'))
ORDER BY key, (organisation_id IS NOT NULL) DESC;

-- name: GetEntitlementForOrgCheck :one
SELECT id, key, description, scope, organisation_id
FROM entitlements
WHERE key = sqlc.arg('key')
  AND (organisation_id IS NULL OR organisation_id = sqlc.arg('organisation_id'))
ORDER BY (organisation_id IS NOT NULL) DESC
LIMIT 1;

-- name: CreateProductPlan :one
INSERT INTO product_plans (id, organisation_id, key, name, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetProductPlan :one
SELECT * FROM product_plans
WHERE id = $1;

-- name: ListProductPlansByOrg :many
SELECT * FROM product_plans
WHERE organisation_id = $1
ORDER BY key;

-- name: UpdateProductPlan :one
UPDATE product_plans
SET name = COALESCE(sqlc.narg('name'), name),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProductPlan :exec
DELETE FROM product_plans
WHERE id = $1
  AND organisation_id = $2;

-- name: DeleteProductPlanFeatures :exec
DELETE FROM product_plan_features
WHERE product_plan_id = $1;

-- name: AddProductPlanFeature :exec
INSERT INTO product_plan_features (product_plan_id, entitlement_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListProductPlanFeatureKeys :many
SELECT e.key
FROM product_plan_features ppf
JOIN entitlements e ON e.id = ppf.entitlement_id
WHERE ppf.product_plan_id = $1
ORDER BY e.key;

-- name: UpsertOrganisationProductPlan :one
INSERT INTO organisation_product_plans (organisation_id, product_plan_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (organisation_id) DO UPDATE
SET product_plan_id = EXCLUDED.product_plan_id,
    updated_at = now()
RETURNING *;

-- name: GetOrganisationProductPlan :one
SELECT * FROM organisation_product_plans
WHERE organisation_id = $1;

-- name: ClearOrganisationProductPlan :exec
DELETE FROM organisation_product_plans
WHERE organisation_id = $1;
