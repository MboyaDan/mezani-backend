-- name: CreateMenuCategory :one
INSERT INTO menu_categories (
    id,
    menu_id,
    name,
    display_order
)
VALUES ($1,$2,$3,$4)
RETURNING *;

-- name: GetMenuCategories :many
SELECT *
FROM menu_categories
WHERE menu_id = $1
ORDER BY display_order;