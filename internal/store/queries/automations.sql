-- name: CreateAutomation :one
INSERT INTO automations (
    id, organisation_id, name, trigger, enabled, conditions, actions
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetAutomation :one
SELECT * FROM automations WHERE id = $1;

-- name: ListAutomations :many
SELECT * FROM automations
WHERE organisation_id = $1
ORDER BY created_at DESC;

-- name: ListEnabledAutomationsByTrigger :many
SELECT * FROM automations
WHERE organisation_id = $1
  AND trigger = $2
  AND enabled = true
ORDER BY created_at ASC;

-- name: UpdateAutomation :one
UPDATE automations SET
    name = COALESCE(sqlc.narg('name'), name),
    trigger = COALESCE(sqlc.narg('trigger'), trigger),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    conditions = COALESCE(sqlc.narg('conditions'), conditions),
    actions = COALESCE(sqlc.narg('actions'), actions),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAutomation :exec
DELETE FROM automations WHERE id = $1;

-- name: CreateAutomationRun :one
INSERT INTO automation_runs (
    id, organisation_id, automation_id, trigger, status, detail, payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListAutomationRuns :many
SELECT * FROM automation_runs
WHERE organisation_id = $1
  AND (sqlc.narg('automation_id')::text IS NULL OR automation_id = sqlc.narg('automation_id'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count');
