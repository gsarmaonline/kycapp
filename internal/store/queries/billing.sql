-- name: CreatePlan :one
INSERT INTO plans (id, key, name, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPlan :one
SELECT * FROM plans
WHERE id = $1;

-- name: GetPlanByKey :one
SELECT * FROM plans
WHERE key = $1;

-- name: ListPlans :many
SELECT * FROM plans
ORDER BY key;

-- name: CreateEntitlement :one
INSERT INTO entitlements (id, key, description, scope, organisation_id)
VALUES ($1, $2, $3, $4, NULL)
RETURNING *;

-- name: GetEntitlementByKey :one
SELECT * FROM entitlements
WHERE key = $1;

-- name: ListEntitlements :many
SELECT * FROM entitlements
WHERE organisation_id IS NULL
ORDER BY scope, key;

-- name: ListEntitlementKeysByPlan :many
SELECT e.key
FROM plan_entitlements pe
JOIN entitlements e ON e.id = pe.entitlement_id
WHERE pe.plan_id = $1
ORDER BY e.key;

-- name: ListEntitlementsByPlan :many
SELECT e.key, e.scope
FROM plan_entitlements pe
JOIN entitlements e ON e.id = pe.entitlement_id
WHERE pe.plan_id = $1
ORDER BY e.scope, e.key;

-- name: ListEntitlementScopesByKeys :many
SELECT key, scope FROM entitlements
WHERE key = ANY(sqlc.arg('keys')::text[])
ORDER BY scope, key;

-- name: DeletePlanEntitlements :exec
DELETE FROM plan_entitlements
WHERE plan_id = $1;

-- name: AddPlanEntitlement :exec
INSERT INTO plan_entitlements (plan_id, entitlement_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListEntitlementIDsByKeys :many
SELECT id, key FROM entitlements
WHERE key = ANY(sqlc.arg('keys')::text[])
ORDER BY key;

-- name: CreateSubscription :one
INSERT INTO subscriptions (id, organisation_id, plan_id, status, current_period_end)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpsertSubscription :one
INSERT INTO subscriptions (id, organisation_id, plan_id, status, current_period_end)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organisation_id) DO UPDATE
SET plan_id = EXCLUDED.plan_id,
    status = EXCLUDED.status,
    current_period_end = EXCLUDED.current_period_end
RETURNING *;

-- name: GetSubscriptionByOrganisation :one
SELECT * FROM subscriptions
WHERE organisation_id = $1;

-- name: DeleteOrganisationEntitlements :exec
DELETE FROM organisation_entitlements
WHERE organisation_id = $1;

-- name: UpsertOrganisationEntitlement :exec
INSERT INTO organisation_entitlements (organisation_id, entitlement_id, effect)
VALUES ($1, $2, $3)
ON CONFLICT (organisation_id, entitlement_id) DO UPDATE
SET effect = EXCLUDED.effect;

-- name: ListOrganisationEntitlementOverrides :many
SELECT e.key, oe.effect
FROM organisation_entitlements oe
JOIN entitlements e ON e.id = oe.entitlement_id
WHERE oe.organisation_id = $1
ORDER BY e.key;
