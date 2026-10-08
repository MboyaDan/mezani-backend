-- name: CreateOrder :one
INSERT INTO orders (
    id,
    table_session_id,
    customer_session_id,
    cart_id,
    status,
    note
)
VALUES ($1,$2,$3,$4,'pending',$5)
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

-- name: GetRecentOrdersByBranch :many
SELECT 
    o.id,
    o.table_session_id,
    o.customer_session_id,
    o.cart_id,
    o.status,
    o.created_at,
    o.note,
    t.table_number
FROM orders o
JOIN table_sessions ts ON ts.id = o.table_session_id
JOIN tables t ON t.id = ts.table_id
WHERE t.branch_id = $1
ORDER BY o.created_at DESC
LIMIT 50;

-- name: GetOrderItemsByOrder :many
SELECT 
    oi.id,
    oi.order_id,
    oi.quantity,
    mi.name,
    mi.price
FROM order_items oi
JOIN menu_items mi ON mi.id = oi.menu_item_id
WHERE oi.order_id = $1;