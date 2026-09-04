-- name: CreateTenant :one
INSERT INTO tenants (
    id,
    name,
    plan
)
VALUES ($1,$2,$3)
RETURNING *;

-- name: GetTenantByID :one
SELECT *
FROM tenants
WHERE id = $1;

-- name: GetTenantSubscriptionStatus :one
-- Used by the enforcement middleware on (nearly) every authenticated
-- request — deliberately narrow rather than SELECT * on the whole row.
SELECT subscription_status, subscription_expires_at
FROM tenants
WHERE id = $1;

-- name: GetTenantByIDForUpdate :one
-- Locks the tenant row for the duration of the transaction — used by
-- BranchService.CreateBranch to serialize concurrent branch-creation
-- attempts for the same tenant, closing the count-then-insert race.
SELECT *
FROM tenants
WHERE id = $1
FOR UPDATE;

-- name: ExtendTenantSubscription :one
-- Takes plan_id, not a free-floating plan name + period. Both `plan` and
-- the renewal period are derived from the matching plans row in this
-- same UPDATE, so it's structurally impossible for a caller to set
-- tenants.plan to a nonexistent/inactive plan, or push
-- subscription_expires_at backward with a negative period — there is no
-- parameter path that bypasses the plans table.
UPDATE tenants
SET subscription_status = 'active',
    plan = p.name,
    subscription_expires_at = GREATEST(NOW(), tenants.subscription_expires_at) + make_interval(days => p.billing_period_days)
FROM plans p
WHERE tenants.id = sqlc.arg(tenant_id)
  AND p.id = sqlc.arg(plan_id)
  AND p.is_active = TRUE
RETURNING tenants.*;
