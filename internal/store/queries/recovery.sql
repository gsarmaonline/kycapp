-- name: CreateRecoveryCredential :one
INSERT INTO recovery_credentials (id, name, token_prefix, token_hash, granted_by, reason, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- GetLiveRecoveryCredentialByHash resolves a credential only while it is usable.
-- Expiry and revocation are enforced in SQL so no caller can forget them.
-- name: GetLiveRecoveryCredentialByHash :one
SELECT * FROM recovery_credentials
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: ListRecoveryCredentials :many
SELECT * FROM recovery_credentials
ORDER BY created_at DESC
LIMIT sqlc.arg('limit');

-- name: RevokeRecoveryCredential :one
UPDATE recovery_credentials
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- name: TouchRecoveryCredentialLastUsed :exec
UPDATE recovery_credentials SET last_used_at = now() WHERE id = $1;
