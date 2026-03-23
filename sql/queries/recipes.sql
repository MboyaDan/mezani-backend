-- name: AddIngredientToMenuItem :exec
INSERT INTO menu_item_ingredients (
    id,
    menu_item_id,
    inventory_item_id,
    quantity_required
)
VALUES ($1, $2, $3, $4);

-- name: GetIngredientsByMenuItem :many
SELECT
    mii.id,
    mii.quantity_required,
    inv.id   AS inventory_item_id,
    inv.name AS ingredient_name,
    inv.stock,
    inv.threshold
FROM menu_item_ingredients mii
JOIN inventory_items inv ON inv.id = mii.inventory_item_id
WHERE mii.menu_item_id = $1;

-- name: UpdateIngredientQuantity :exec
UPDATE menu_item_ingredients
SET quantity_required = $3
WHERE menu_item_id = $1
  AND inventory_item_id = $2;

-- name: RemoveIngredientFromMenuItem :exec
DELETE FROM menu_item_ingredients
WHERE menu_item_id = $1
  AND inventory_item_id = $2;

-- name: GetMenuItemsByIngredient :many
SELECT
    mi.id,
    mi.name,
    mi.available,
    mi.sold_out
FROM menu_item_ingredients mii
JOIN menu_items mi ON mi.id = mii.menu_item_id
WHERE mii.inventory_item_id = $1;