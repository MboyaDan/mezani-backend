-- name: GetOrdersBySession :many
SELECT o.*
FROM orders o
WHERE o.customer_session_id IN (
    SELECT cs.id
    FROM customer_sessions cs
    WHERE cs.table_session_id = $1
);

-- name: MarkOrdersPaid :exec
UPDATE orders
SET status = 'paid'
WHERE id = ANY($1::uuid[]);