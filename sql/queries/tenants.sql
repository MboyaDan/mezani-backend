-- name: CreateTenant :one
INSERT INTO tenants (
    id,
    name,
    plan
)
VALUES ($1,$2,$3)
RETURNING *;

-- name: GetTenantByID :one
SELECT *
FROM tenants
WHERE id = $1;