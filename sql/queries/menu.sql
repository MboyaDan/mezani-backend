-- name: CreateMenu :one
INSERT INTO menus (
    id,
    branch_id
)
VALUES ($1,$2)
RETURNING *;

-- name: GetMenu :one
SELECT *
FROM menus
WHERE id = $1;

-- name: GetBranchMenus :many
SELECT *
FROM menus
WHERE branch_id = $1
ORDER BY created_at DESC;