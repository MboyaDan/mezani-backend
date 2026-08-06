-- name: GetTableSessionOwnership :one
-- Derives the true tenant_id/branch_id for a table session by walking
-- table_session -> table -> branch. Used so payments.tenant_id/branch_id
-- are always computed from the table session itself rather than trusted
-- from caller-supplied values, which could otherwise drift out of sync
-- (e.g. a staff member's own JWT tenant/branch not matching the table
-- session they're being asked to act on).
SELECT b.tenant_id, t.branch_id
FROM table_sessions ts
JOIN tables t ON t.id = ts.table_id
JOIN branches b ON b.id = t.branch_id
WHERE ts.id = $1;

-- name: GetTableSessionOutstandingTotal :one
-- Server-side source of truth for what's owed on a bill. Never trust a
-- client-supplied amount for payment initiation — always recompute here.
SELECT COALESCE(SUM(oi.quantity * mi.price), 0)::float8 AS total
FROM orders o
JOIN order_items oi ON oi.order_id = o.id
JOIN menu_items mi ON mi.id = oi.menu_item_id
WHERE o.table_session_id = $1
  AND o.status = 'served';

-- name: CreatePayment :one
INSERT INTO payments (
    id, tenant_id, branch_id, table_session_id,
    method, amount, initiated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments
WHERE id = $1;

-- name: GetPendingPaymentByTableSession :one
SELECT * FROM payments
WHERE table_session_id = $1 AND status = 'pending';

-- name: GetPendingPaymentsByBranch :many
SELECT * FROM payments
WHERE branch_id = $1 AND status = 'pending'
ORDER BY created_at ASC;

-- name: ConfirmPayment :one
UPDATE payments
SET status = 'confirmed',
    confirmed_by = $2,
    confirmed_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: CancelPayment :one
UPDATE payments
SET status = 'cancelled'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: GetPaymentsByTableSession :many
SELECT * FROM payments
WHERE table_session_id = $1
ORDER BY created_at DESC;