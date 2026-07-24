-- name: CreateOrganisationWebhook :one
INSERT INTO organisation_webhooks (
    id, organisation_id, name, url, secret, status, body_template
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetOrganisationWebhook :one
SELECT * FROM organisation_webhooks
WHERE id = $1;

-- name: GetOrganisationWebhookForOrg :one
SELECT * FROM organisation_webhooks
WHERE id = $1 AND organisation_id = $2;

-- name: ListOrganisationWebhooks :many
SELECT * FROM organisation_webhooks
WHERE organisation_id = $1
ORDER BY name, created_at;

-- name: UpdateOrganisationWebhook :one
UPDATE organisation_webhooks
SET name = sqlc.arg(name),
    url = sqlc.arg(url),
    secret = CASE
        WHEN sqlc.arg(secret)::text = '' THEN secret
        ELSE sqlc.arg(secret)::text
    END,
    status = sqlc.arg(status),
    body_template = sqlc.arg(body_template),
    updated_at = now()
WHERE id = sqlc.arg(id) AND organisation_id = sqlc.arg(organisation_id)
RETURNING *;

-- name: DeleteOrganisationWebhook :exec
DELETE FROM organisation_webhooks
WHERE id = $1 AND organisation_id = $2;
