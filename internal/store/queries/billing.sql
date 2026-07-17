-- name: GetPlanByKey :one
SELECT * FROM plans
WHERE key = $1;

-- name: CreateSubscription :one
INSERT INTO subscriptions (id, organisation_id, plan_id, status, current_period_end)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSubscriptionByOrganisation :one
SELECT * FROM subscriptions
WHERE organisation_id = $1;
