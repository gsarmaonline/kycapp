-- name: InsertActivityEvent :one
INSERT INTO activity_events (
    id,
    organisation_id,
    organisation_slug,
    organisation_name,
    actor_type,
    actor_id,
    actor_label,
    action,
    resource_type,
    resource_id,
    summary,
    payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: ListActivityEventsByOrg :many
SELECT *
FROM activity_events
WHERE organisation_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: DeleteActivityEventsByOrg :exec
DELETE FROM activity_events
WHERE organisation_id = $1;
