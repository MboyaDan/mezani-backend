
-- name: DeductStockForOrder :exec
UPDATE inventory_items inv
SET stock = inv.stock - (mii.quantity_required * oi.quantity)
FROM order_items oi
JOIN menu_item_ingredients mii ON mii.menu_item_id = oi.menu_item_id
WHERE inv.id = mii.inventory_item_id
  AND oi.order_id = $1
  AND inv.branch_id = $2;

-- name: AutoMarkSoldOut :exec
UPDATE menu_items
SET sold_out = TRUE
WHERE id IN (
    SELECT DISTINCT mii.menu_item_id
    FROM menu_item_ingredients mii
    JOIN inventory_items inv ON inv.id = mii.inventory_item_id
    WHERE inv.stock <= 0
      AND inv.branch_id = $1
);

-- name: RestoreStockForOrder :exec
UPDATE inventory_items inv
SET stock = inv.stock + (mii.quantity_required * oi.quantity)
FROM order_items oi
JOIN menu_item_ingredients mii ON mii.menu_item_id = oi.menu_item_id
WHERE inv.id = mii.inventory_item_id
  AND oi.order_id = $1
  AND inv.branch_id = $2;
