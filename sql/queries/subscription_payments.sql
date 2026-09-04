-- name: CreateSubscriptionPayment :one
INSERT INTO subscription_payments (
    id, tenant_id, plan_id, amount, currency, paystack_reference, initiated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetSubscriptionPaymentByReference :one
SELECT * FROM subscription_payments
WHERE paystack_reference = $1;

-- name: MarkSubscriptionPaymentSuccess :one
UPDATE subscription_payments
SET status = 'success',
    verified_at = NOW()
WHERE paystack_reference = $1 AND status = 'pending'
RETURNING *;

-- name: MarkSubscriptionPaymentFailed :one
UPDATE subscription_payments
SET status = 'failed',
    verified_at = NOW()
WHERE paystack_reference = $1 AND status = 'pending'
RETURNING *;

-- name: ListSubscriptionPaymentsByTenant :many
SELECT * FROM subscription_payments
WHERE tenant_id = $1
ORDER BY created_at DESC;