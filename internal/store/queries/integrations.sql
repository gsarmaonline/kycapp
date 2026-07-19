-- name: UpsertOrganisationIntegration :one
INSERT INTO organisation_integrations (
    organisation_id, provider, status, secret_key, public_key, updated_at
)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (organisation_id, provider) DO UPDATE
SET status = EXCLUDED.status,
    secret_key = CASE
        WHEN EXCLUDED.secret_key = '' THEN organisation_integrations.secret_key
        ELSE EXCLUDED.secret_key
    END,
    public_key = CASE
        WHEN EXCLUDED.public_key = '' THEN organisation_integrations.public_key
        ELSE EXCLUDED.public_key
    END,
    updated_at = now()
RETURNING *;

-- name: GetOrganisationIntegration :one
SELECT * FROM organisation_integrations
WHERE organisation_id = $1 AND provider = $2;

-- name: ListOrganisationIntegrations :many
SELECT * FROM organisation_integrations
WHERE organisation_id = $1
ORDER BY provider;

-- name: DeleteOrganisationIntegration :exec
DELETE FROM organisation_integrations
WHERE organisation_id = $1 AND provider = $2;
