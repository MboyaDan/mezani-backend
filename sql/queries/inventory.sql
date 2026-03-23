-- name: CreateInventoryItem :one
INSERT INTO inventory_items (
    id,
    branch_id,
    name,
    stock,
    threshold
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateStock :exec
UPDATE inventory_items
SET stock = stock - $2
WHERE id = $1
  AND branch_id = $3;

-- name: GetLowStockItems :many
SELECT *
FROM inventory_items
WHERE branch_id = $1
  AND stock <= threshold;

-- name: GetInventoryItemsByBranch :many
SELECT *
FROM inventory_items
WHERE branch_id = $1
ORDER BY name ASC;

-- name: SetStock :one
UPDATE inventory_items
SET stock = $2
WHERE id = $1
  AND branch_id = $3
RETURNING *;

-- name: GetInventoryItemByID :one
SELECT *
FROM inventory_items
WHERE id = $1
  AND branch_id = $2;