-- name: CreateOrganisationInboundWebhook :one
INSERT INTO organisation_inbound_webhooks (
    id, organisation_id, name, secret, status
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetOrganisationInboundWebhook :one
SELECT * FROM organisation_inbound_webhooks
WHERE id = $1;

-- name: GetOrganisationInboundWebhookForOrg :one
SELECT * FROM organisation_inbound_webhooks
WHERE id = $1 AND organisation_id = $2;

-- name: ListOrganisationInboundWebhooks :many
SELECT * FROM organisation_inbound_webhooks
WHERE organisation_id = $1
ORDER BY created_at DESC;

-- name: UpdateOrganisationInboundWebhook :one
UPDATE organisation_inbound_webhooks SET
    name = $3,
    secret = $4,
    status = $5,
    updated_at = now()
WHERE id = $1 AND organisation_id = $2
RETURNING *;

-- name: DeleteOrganisationInboundWebhook :exec
DELETE FROM organisation_inbound_webhooks
WHERE id = $1 AND organisation_id = $2;
