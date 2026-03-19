-- name: CreateStaffUser :one
INSERT INTO staff_users (
    id,
    tenant_id,
    branch_id,
    name,
    email,
    password_hash,
    role,
    created_by        
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING *;

-- name: GetStaffByEmail :one
SELECT *
FROM staff_users
WHERE email = $1;

-- name: GetBranchStaff :many
SELECT *
FROM staff_users
WHERE branch_id = $1;