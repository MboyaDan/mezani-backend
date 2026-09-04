-- ============================================================
-- PLANS
-- ============================================================
CREATE TABLE plans (
    id                  UUID PRIMARY KEY,
    name                TEXT UNIQUE NOT NULL,
    display_name        TEXT NOT NULL,
    price_kes           NUMERIC NOT NULL,
    billing_period_days INT NOT NULL DEFAULT 30,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMP DEFAULT NOW(),
    CONSTRAINT plans_price_check CHECK (
        price_kes > 0
        AND price_kes NOT IN ('NaN'::numeric, 'Infinity'::numeric, '-Infinity'::numeric)
    ),
    CONSTRAINT plans_billing_period_check CHECK (billing_period_days > 0)
);

INSERT INTO plans (id, name, display_name, price_kes, billing_period_days) VALUES
    (gen_random_uuid(), 'tier1', 'Basic',    3000,  30),
    (gen_random_uuid(), 'tier2', 'Standard', 7000,  30),
    (gen_random_uuid(), 'tier3', 'Premium',  15000, 30);

-- ============================================================
-- SUBSCRIPTION PAYMENTS
-- ============================================================
CREATE TABLE subscription_payments (
    id                  UUID PRIMARY KEY,
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    plan_id             UUID NOT NULL REFERENCES plans(id),
    amount              NUMERIC NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'KES',
    paystack_reference  TEXT UNIQUE NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    initiated_by        UUID REFERENCES staff_users(id),
    created_at          TIMESTAMP DEFAULT NOW(),
    verified_at         TIMESTAMP,
    CONSTRAINT subscription_payments_status_check CHECK (status IN ('pending', 'success', 'failed')),
    CONSTRAINT subscription_payments_amount_check CHECK (
        amount > 0
        AND amount NOT IN ('NaN'::numeric, 'Infinity'::numeric, '-Infinity'::numeric)
    )
);

CREATE INDEX idx_subscription_payments_tenant_id ON subscription_payments (tenant_id);
CREATE INDEX idx_subscription_payments_status    ON subscription_payments (status);

-- ============================================================
-- TENANTS — subscription tracking
-- ============================================================
ALTER TABLE tenants ADD COLUMN subscription_status TEXT NOT NULL DEFAULT 'trialing';
ALTER TABLE tenants ADD COLUMN subscription_expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '14 days');

ALTER TABLE tenants ADD CONSTRAINT tenants_subscription_status_check
    CHECK (subscription_status IN ('trialing', 'active', 'expired', 'cancelled'));

UPDATE tenants
SET subscription_expires_at = created_at + INTERVAL '14 days';

-- ============================================================
-- RLS — same default-deny posture as every other table
-- ============================================================
ALTER TABLE plans                 ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_payments ENABLE ROW LEVEL SECURITY;