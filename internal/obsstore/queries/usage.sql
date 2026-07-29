-- name: IncrUsageCounter :one
INSERT INTO usage_counters (
    organisation_id,
    meter_key,
    period_start,
    dim1_key,
    dim1_value,
    dim2_key,
    dim2_value,
    count,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, now()
)
ON CONFLICT (
    organisation_id,
    meter_key,
    period_start,
    dim1_key,
    dim1_value,
    dim2_key,
    dim2_value
) DO UPDATE SET
    count = usage_counters.count + EXCLUDED.count,
    updated_at = now()
RETURNING *;

-- name: ListUsageCountersByOrgPeriod :many
SELECT *
FROM usage_counters
WHERE organisation_id = sqlc.arg(organisation_id)
  AND period_start >= sqlc.arg(from_period)
  AND period_start < sqlc.arg(to_period)
ORDER BY meter_key, dim1_key, dim1_value, dim2_key, dim2_value;

-- name: DeleteUsageCountersByOrg :exec
DELETE FROM usage_counters
WHERE organisation_id = $1;
