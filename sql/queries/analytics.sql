-- name: GetDailySales :one
-- Fix: range comparison instead of DATE() so the index on created_at is used
SELECT
    COALESCE(SUM(mi.price * oi.quantity), 0) AS total_sales
FROM orders o
JOIN order_items oi ON o.id = oi.order_id
JOIN menu_items mi ON oi.menu_item_id = mi.id
JOIN table_sessions ts ON o.table_session_id = ts.id
JOIN tables t ON ts.table_id = t.id
WHERE o.status = 'paid'
AND o.created_at >= CURRENT_DATE
AND o.created_at <  CURRENT_DATE + INTERVAL '1 day'
AND t.branch_id = $1;

-- name: GetPopularItems :many
-- Fix: filter to meaningful statuses only
SELECT
    mi.name,
    SUM(oi.quantity) AS total_sold
FROM order_items oi
JOIN orders o ON oi.order_id = o.id
JOIN menu_items mi ON oi.menu_item_id = mi.id
JOIN table_sessions ts ON o.table_session_id = ts.id
JOIN tables t ON ts.table_id = t.id
WHERE t.branch_id = $1
AND o.status IN ('paid', 'served')      -- exclude pending/cancelled
GROUP BY mi.name
ORDER BY total_sold DESC
LIMIT 5;

-- name: GetPeakHours :many
-- Fix: filter to meaningful statuses only
SELECT
    EXTRACT(HOUR FROM o.created_at) AS hour,
    COUNT(*) AS order_count
FROM orders o
JOIN table_sessions ts ON o.table_session_id = ts.id
JOIN tables t ON ts.table_id = t.id
WHERE t.branch_id = $1
AND o.status IN ('paid', 'served')      -- exclude pending/cancelled
GROUP BY hour
ORDER BY order_count DESC;

-- name: GetReturningCustomers :one
-- Fix: filter to meaningful statuses only
SELECT COUNT(*) FROM (
    SELECT o.customer_session_id
    FROM orders o
    JOIN table_sessions ts ON o.table_session_id = ts.id
    JOIN tables t ON ts.table_id = t.id
    WHERE t.branch_id = $1
    AND o.status IN ('paid', 'served')  -- only count real completed visits
    GROUP BY o.customer_session_id
    HAVING COUNT(*) > 1
) AS returning_customers;