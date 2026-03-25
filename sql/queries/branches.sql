-- name: CreateBranch :one
INSERT INTO branches (
 id,
 tenant_id,
 name,
 location
)
VALUES ($1,$2,$3,$4)
RETURNING *;

-- name: GetBranchesByTenant :many
SELECT *
FROM branches
WHERE tenant_id = $1;

-- name: GetBranchByID :one
SELECT id, tenant_id, name, location, created_at
FROM branches
WHERE id = $1;