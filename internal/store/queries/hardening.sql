-- name: CreateAPIKey :one
INSERT INTO api_keys (id, name, key_prefix, key_hash, organisation_id, scopes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAPIKey :one
SELECT * FROM api_keys
WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: ListAPIKeys :many
SELECT * FROM api_keys
WHERE organisation_id IS NULL
ORDER BY created_at DESC;

-- name: ListAPIKeysByOrg :many
SELECT * FROM api_keys
WHERE organisation_id = $1
ORDER BY created_at DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- name: InsertAuditEvent :one
INSERT INTO audit_events (id, actor, method, path, status_code, organisation_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditEvents :many
SELECT * FROM audit_events
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');
