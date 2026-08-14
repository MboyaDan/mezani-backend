-- name: GetPlatformOverview :one
-- Top-level summary card. Each figure is an independent scalar subquery,
-- so there's no join fan-out risk at all.
SELECT
    (SELECT COUNT(*) FROM tenants)::int                                        AS tenant_count,
    (SELECT COUNT(*) FROM branches)::int                                       AS branch_count,
    (SELECT COUNT(*) FROM staff_users)::int                                    AS staff_count,
    (SELECT COALESCE(SUM(amount), 0) FROM payments WHERE status = 'confirmed')::float8 AS total_platform_revenue;

-- name: ListTenantsWithStats :many
-- Per-tenant breakdown. branch_count/staff_count/total_revenue are each
-- computed in their own GROUP BY subquery BEFORE joining to tenants —
-- joining the raw branches/staff_users/payments tables directly here
-- instead would multiply payment rows by the branch/staff fan-out and
-- silently inflate total_revenue.
SELECT
    t.id,
    t.name,
    t.plan,
    t.created_at,
    COALESCE(branch_counts.count, 0)::int   AS branch_count,
    COALESCE(staff_counts.count, 0)::int    AS staff_count,
    COALESCE(revenue.total, 0)::float8      AS total_revenue
FROM tenants t
LEFT JOIN (
    SELECT tenant_id, COUNT(*) AS count
    FROM branches
    GROUP BY tenant_id
) branch_counts ON branch_counts.tenant_id = t.id
LEFT JOIN (
    SELECT tenant_id, COUNT(*) AS count
    FROM staff_users
    GROUP BY tenant_id
) staff_counts ON staff_counts.tenant_id = t.id
LEFT JOIN (
    SELECT tenant_id, SUM(amount) AS total
    FROM payments
    WHERE status = 'confirmed'
    GROUP BY tenant_id
) revenue ON revenue.tenant_id = t.id
ORDER BY t.created_at DESC;

-- name: GetTenantByIDForAdmin :one
-- Superadmin-scoped tenant lookup (no tenant-isolation check needed —
-- the caller IS the platform, not a tenant staff member).
SELECT * FROM tenants
WHERE id = $1;