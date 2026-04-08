-- name: CreateMenuItem :one
INSERT INTO menu_items (
    id,
    category_id,
    name,
    description,
    price
)
VALUES ($1,$2,$3,$4,$5)
RETURNING *;

-- name: GetCategoryItems :many
SELECT *
FROM menu_items
WHERE category_id = $1
AND available = TRUE
AND sold_out = FALSE;

-- name: UpdateMenuItemPrice :exec
UPDATE menu_items
SET price = $2
WHERE id = $1;



-- name: SetMenuItemSoldOut :exec
UPDATE menu_items
SET 
    sold_out = $2,
    available = CASE 
        WHEN $2 = true THEN false
        ELSE available
    END
WHERE id = $1;

-- name: SetMenuItemAvailable :exec
UPDATE menu_items 
SET sold_out = false, available = true 
WHERE id = $1;

-- name: SetDailySpecial :exec
UPDATE menu_items
SET is_special = $2
WHERE id = $1;