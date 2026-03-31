-- name: CreateTable :one
INSERT INTO tables (
    id,
    branch_id,
    table_number
)
VALUES ($1,$2,$3)
RETURNING *;

-- name: GetTablesByBranch :many
SELECT *
FROM tables
WHERE branch_id = $1;

-- name: GetTable :one
SELECT * FROM tables WHERE id = $1;