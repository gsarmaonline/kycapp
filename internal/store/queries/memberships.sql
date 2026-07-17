-- name: CreateMembership :one
INSERT INTO memberships (id, organisation_id, user_id, role_id, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM memberships
WHERE id = $1;

-- name: ListMembershipsByOrganisation :many
SELECT
  m.*,
  u.email AS user_email,
  u.name AS user_name,
  r.key AS role_key
FROM memberships m
JOIN users u ON u.id = m.user_id
JOIN roles r ON r.id = m.role_id
WHERE m.organisation_id = $1
ORDER BY m.created_at, m.id;

-- name: ListMembershipsByUser :many
SELECT
  m.*,
  o.name AS organisation_name,
  o.slug AS organisation_slug,
  o.status AS organisation_status,
  r.key AS role_key
FROM memberships m
JOIN organisations o ON o.id = m.organisation_id
JOIN roles r ON r.id = m.role_id
WHERE m.user_id = $1
ORDER BY m.created_at, m.id;

-- name: UpdateMembership :one
UPDATE memberships
SET
  role_id = COALESCE(sqlc.narg('role_id'), role_id),
  status = COALESCE(sqlc.narg('status'), status)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: AcceptMembership :one
UPDATE memberships
SET status = 'active'
WHERE id = $1 AND status = 'invited'
RETURNING *;

-- name: RevokeMembership :one
UPDATE memberships
SET status = 'revoked'
WHERE id = $1
RETURNING *;
