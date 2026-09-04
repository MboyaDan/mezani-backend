-- name: ListActivePlans :many
SELECT * FROM plans
WHERE is_active = TRUE
ORDER BY price_kes ASC;

-- name: GetPlanByName :one
SELECT * FROM plans
WHERE name = $1 AND is_active = TRUE;

-- name: GetPlanByID :one
SELECT * FROM plans
WHERE id = $1;