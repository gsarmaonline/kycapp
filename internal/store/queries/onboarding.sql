-- name: GetOrganisationOnboarding :one
SELECT organisation_id, dismissed_at, updated_at
FROM organisation_onboarding
WHERE organisation_id = $1;

-- name: UpsertOrganisationOnboardingDismissed :one
INSERT INTO organisation_onboarding (organisation_id, dismissed_at, updated_at)
VALUES ($1, now(), now())
ON CONFLICT (organisation_id) DO UPDATE
SET dismissed_at = now(),
    updated_at = now()
RETURNING organisation_id, dismissed_at, updated_at;

-- name: CountProductFeaturesByOrg :one
SELECT COUNT(*)::bigint
FROM entitlements
WHERE organisation_id = $1 AND scope = 'product';

-- name: CountProductPlansByOrg :one
SELECT COUNT(*)::bigint
FROM product_plans
WHERE organisation_id = $1;

-- name: CountAutomationsByOrg :one
SELECT COUNT(*)::bigint
FROM automations
WHERE organisation_id = $1;

-- name: CountAppUsersByOrg :one
SELECT COUNT(*)::bigint
FROM app_users
WHERE organisation_id = $1;

-- name: CountActiveAPIKeysByOrg :one
SELECT COUNT(*)::bigint
FROM api_keys
WHERE organisation_id = $1 AND revoked_at IS NULL;
