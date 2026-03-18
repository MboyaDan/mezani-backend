-- name: CreateOrder :one
INSERT INTO orders (
    id,
    table_session_id,
    customer_session_id,
    cart_id,
    status
)
VALUES ($1,$2,$3,$4,'pending')
RETURNING *;


-- name: CreateOrderItem :one
INSERT INTO order_items (
    id,
    order_id,
    menu_item_id,
    quantity
)
VALUES ($1,$2,$3,$4)
RETURNING *;


-- name: GetOrdersByTableSession :many
SELECT *
FROM orders
WHERE table_session_id = $1
ORDER BY created_at DESC;


-- name: GetOrderByID :one
SELECT *
FROM orders
WHERE id = $1;


-- SAFE + IDEMPOTENT STATUS UPDATE
-- name: UpdateOrderStatusSafe :one
UPDATE orders
SET status = $3
WHERE id = $1
AND status = $2
RETURNING *;

-- name: CloseOrderBill :one
UPDATE orders
SET status = 'closed'
WHERE id = $1
RETURNING *;