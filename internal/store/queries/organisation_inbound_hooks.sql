-- name: GetOrganisationInboundHook :one
SELECT organisation_id, secret, status, updated_at
FROM organisation_inbound_hooks
WHERE organisation_id = $1;

-- name: UpsertOrganisationInboundHook :one
INSERT INTO organisation_inbound_hooks (organisation_id, secret, status, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (organisation_id) DO UPDATE SET
    secret = EXCLUDED.secret,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING organisation_id, secret, status, updated_at;
