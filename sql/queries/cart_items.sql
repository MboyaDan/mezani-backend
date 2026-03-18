-- name: AddCartItem :one
INSERT INTO cart_items (
    id,
    cart_id,
    menu_item_id,
    quantity,
    added_by
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;


-- name: GetCartItems :many
SELECT 
    ci.id,
    ci.cart_id,
    ci.menu_item_id,
    ci.quantity,
    mi.name
FROM cart_items ci
JOIN menu_items mi ON ci.menu_item_id = mi.id
WHERE ci.cart_id = $1;


-- name: ClearCartItems :exec
DELETE FROM cart_items
WHERE cart_id = $1;