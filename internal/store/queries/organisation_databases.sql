-- name: CreateOrganisationDatabase :one
INSERT INTO organisation_databases (
    id, organisation_id, name, driver, host, port, database_name,
    username, password, ssl_mode, status, last_error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetOrganisationDatabase :one
SELECT * FROM organisation_databases
WHERE id = $1;

-- name: GetOrganisationDatabaseForOrg :one
SELECT * FROM organisation_databases
WHERE id = $1 AND organisation_id = $2;

-- name: ListOrganisationDatabases :many
SELECT * FROM organisation_databases
WHERE organisation_id = $1
ORDER BY name, created_at;

-- name: UpdateOrganisationDatabase :one
UPDATE organisation_databases
SET name = sqlc.arg(name),
    host = sqlc.arg(host),
    port = sqlc.arg(port),
    database_name = sqlc.arg(database_name),
    username = sqlc.arg(username),
    password = CASE
        WHEN sqlc.arg(password)::text = '' THEN password
        ELSE sqlc.arg(password)::text
    END,
    ssl_mode = sqlc.arg(ssl_mode),
    updated_at = now()
WHERE id = sqlc.arg(id) AND organisation_id = sqlc.arg(organisation_id)
RETURNING *;

-- name: UpdateOrganisationDatabaseCheck :one
UPDATE organisation_databases
SET status = sqlc.arg(status),
    last_checked_at = now(),
    last_error = sqlc.arg(last_error),
    updated_at = now()
WHERE id = sqlc.arg(id) AND organisation_id = sqlc.arg(organisation_id)
RETURNING *;

-- name: DeleteOrganisationDatabase :exec
DELETE FROM organisation_databases
WHERE id = $1 AND organisation_id = $2;
