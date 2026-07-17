-- name: CreateEmailTemplate :one
INSERT INTO email_templates (
    id, organisation_id, key, name, description,
    subject, body_text, body_html, status, is_system
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetEmailTemplate :one
SELECT * FROM email_templates WHERE id = $1;

-- name: GetEmailTemplateByOrgKey :one
SELECT * FROM email_templates
WHERE organisation_id = $1 AND key = $2;

-- name: ListEmailTemplates :many
SELECT * FROM email_templates
WHERE organisation_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY is_system DESC, key ASC;

-- name: UpdateEmailTemplate :one
UPDATE email_templates SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    subject = COALESCE(sqlc.narg('subject'), subject),
    body_text = COALESCE(sqlc.narg('body_text'), body_text),
    body_html = COALESCE(sqlc.narg('body_html'), body_html),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveEmailTemplate :one
UPDATE email_templates
SET status = 'archived', updated_at = now()
WHERE id = $1
RETURNING *;
