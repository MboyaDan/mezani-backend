-- name: CreateCustomerSession :one
INSERT INTO customer_sessions (
    id,
    table_session_id,
    name
)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCustomerSession :one
SELECT *
FROM customer_sessions
WHERE id = $1;

-- name: ListCustomersByTableSession :many
SELECT *
FROM customer_sessions
WHERE table_session_id = $1
ORDER BY created_at ASC;