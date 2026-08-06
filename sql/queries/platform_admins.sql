-- name: CreatePlatformAdmin :one
INSERT INTO platform_admins (id, name, email, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPlatformAdminByEmail :one
SELECT * FROM platform_admins
WHERE email = $1;

-- name: GetPlatformAdminByID :one
SELECT * FROM platform_admins
WHERE id = $1;
