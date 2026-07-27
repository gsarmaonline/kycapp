-- name: CreateFeatureFlag :one
INSERT INTO feature_flags (id, organisation_id, key, description, enabled, rollout_percentage)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetFeatureFlag :one
SELECT * FROM feature_flags
WHERE id = $1;

-- name: GetFeatureFlagByOrgKey :one
SELECT * FROM feature_flags
WHERE organisation_id = $1
  AND key = $2;

-- name: ListFeatureFlagsByOrg :many
SELECT * FROM feature_flags
WHERE organisation_id = $1
ORDER BY key;

-- name: UpdateFeatureFlag :one
UPDATE feature_flags
SET description = COALESCE(sqlc.narg('description'), description),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    rollout_percentage = COALESCE(sqlc.narg('rollout_percentage'), rollout_percentage),
    updated_at = now()
WHERE id = $1
  AND organisation_id = $2
RETURNING *;

-- name: DeleteFeatureFlag :exec
DELETE FROM feature_flags
WHERE id = $1
  AND organisation_id = $2;

-- name: ListFeatureFlagOverrides :many
SELECT * FROM feature_flag_overrides
WHERE feature_flag_id = $1
ORDER BY subject_id;

-- name: DeleteFeatureFlagOverrides :exec
DELETE FROM feature_flag_overrides
WHERE feature_flag_id = $1;

-- name: UpsertFeatureFlagOverride :exec
INSERT INTO feature_flag_overrides (feature_flag_id, subject_id, effect)
VALUES ($1, $2, $3)
ON CONFLICT (feature_flag_id, subject_id) DO UPDATE
SET effect = EXCLUDED.effect;

-- name: GetFeatureFlagOverride :one
SELECT * FROM feature_flag_overrides
WHERE feature_flag_id = $1
  AND subject_id = $2;
