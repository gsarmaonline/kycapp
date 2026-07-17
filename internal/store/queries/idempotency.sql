-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE key = $1;

-- name: InsertIdempotencyKey :one
INSERT INTO idempotency_keys (key, request_hash, response_status, response_body)
VALUES ($1, $2, $3, $4)
RETURNING *;
