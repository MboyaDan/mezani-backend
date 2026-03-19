-- name: GetDailySales :one
SELECT
    COALESCE(SUM(mi.price * oi.quantity), 0) AS total_sales
FROM orders o
JOIN order_items oi ON o.id = oi.order_id
JOIN menu_items mi ON oi.menu_item_id = mi.id
WHERE o.status = 'paid'
AND DATE(o.created_at) = CURRENT_DATE;

-- name: GetPopularItems :many
SELECT
    mi.name,
    SUM(oi.quantity) AS total_sold
FROM order_items oi
JOIN menu_items mi ON oi.menu_item_id = mi.id
GROUP BY mi.name
ORDER BY total_sold DESC
LIMIT 5;

-- name: GetPeakHours :many
SELECT
    EXTRACT(HOUR FROM o.created_at) AS hour,
    COUNT(*) AS order_count
FROM orders o
GROUP BY hour
ORDER BY order_count DESC;

-- name: GetReturningCustomers :one
SELECT COUNT(*) FROM (
    SELECT customer_session_id
    FROM orders
    GROUP BY customer_session_id
    HAVING COUNT(*) > 1
) AS returning_customers;