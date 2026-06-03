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
AND o.status IN ('paid', 'served')
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
AND o.status IN ('paid', 'served')
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
    AND o.status IN ('paid', 'served')
    GROUP BY o.customer_session_id
    HAVING COUNT(*) > 1
) AS returning_customers;

-- name: GetAIBranchContext :one
SELECT
    COUNT(DISTINCT o.id)                                            AS total_orders,
    COUNT(DISTINCT ts.id)                                           AS total_sessions,
    COALESCE(SUM(
        (SELECT SUM(mi.price * oi.quantity)
         FROM order_items oi
         JOIN menu_items mi ON mi.id = oi.menu_item_id
         WHERE oi.order_id = o.id
           AND o.status IN ('paid', 'served'))
    ), 0)                                                           AS total_revenue,
    COUNT(DISTINCT CASE WHEN o.status = 'pending'   THEN o.id END) AS pending_orders,
    COUNT(DISTINCT CASE WHEN o.status = 'preparing' THEN o.id END) AS preparing_orders,
    COUNT(DISTINCT CASE WHEN o.status = 'ready'     THEN o.id END) AS ready_orders
FROM orders o
JOIN table_sessions ts ON ts.id = o.table_session_id
JOIN tables t          ON t.id  = ts.table_id
WHERE t.branch_id  = $1
  AND o.created_at >= NOW() - INTERVAL '7 days';

-- name: GetAITopItems :many
SELECT
    mi.name,
    SUM(oi.quantity)::int           AS total_sold,
    SUM(mi.price * oi.quantity)     AS revenue
FROM order_items oi
JOIN menu_items mi     ON mi.id = oi.menu_item_id
JOIN orders o          ON o.id  = oi.order_id
JOIN table_sessions ts ON ts.id = o.table_session_id
JOIN tables t          ON t.id  = ts.table_id
WHERE t.branch_id  = $1
  AND o.status IN ('paid', 'served')
  AND o.created_at >= NOW() - INTERVAL '7 days'
GROUP BY mi.name
ORDER BY total_sold DESC
LIMIT 5;

-- name: GetAIPeakHours :many
SELECT
    EXTRACT(HOUR FROM o.created_at)::int AS hour,
    COUNT(*)::int                         AS order_count
FROM orders o
JOIN table_sessions ts ON ts.id = o.table_session_id
JOIN tables t          ON t.id  = ts.table_id
WHERE t.branch_id  = $1
  AND o.status IN ('paid', 'served')
  AND o.created_at >= NOW() - INTERVAL '7 days'
GROUP BY hour
ORDER BY order_count DESC
LIMIT 3;

-- name: GetAILowStockItems :many
SELECT name, stock, threshold
FROM inventory_items
WHERE branch_id = $1
  AND stock <= threshold
ORDER BY stock ASC
LIMIT 10;

-- name: GetAIStaffCount :one
SELECT COUNT(*)::int AS count
FROM staff_users
WHERE tenant_id = $1
  AND (branch_id = $2 OR branch_id IS NULL);