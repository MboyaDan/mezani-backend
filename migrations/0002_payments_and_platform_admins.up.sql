-- ============================================================
-- PAYMENTS
-- One row per payment attempt against a table's bill.
-- method distinguishes cash vs mpesa (mpesa fields added in a
-- later migration when the gateway integration lands).
-- ============================================================
CREATE TABLE payments (
    id                UUID PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    branch_id         UUID NOT NULL REFERENCES branches(id),
    table_session_id  UUID NOT NULL REFERENCES table_sessions(id),
    method            TEXT NOT NULL,               -- 'cash' | 'mpesa'
    status            TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'confirmed' | 'failed' | 'cancelled'
    amount            NUMERIC NOT NULL,
    mpesa_receipt     TEXT,
    initiated_by      UUID REFERENCES staff_users(id), -- nullable: customer-initiated has no staff row
    confirmed_by      UUID REFERENCES staff_users(id),
    created_at        TIMESTAMP DEFAULT NOW(),
    confirmed_at      TIMESTAMP,
    CONSTRAINT payments_method_check CHECK (method IN ('cash', 'mpesa')),
    CONSTRAINT payments_status_check CHECK (status IN ('pending', 'confirmed', 'failed', 'cancelled')),
    CONSTRAINT payments_amount_check CHECK (
        amount > 0
        AND amount NOT IN (
            'NaN'::numeric,
            'Infinity'::numeric,
            '-Infinity'::numeric
        )
    )
);

CREATE INDEX idx_payments_table_session_id ON payments (table_session_id);
CREATE INDEX idx_payments_tenant_id        ON payments (tenant_id);
CREATE INDEX idx_payments_branch_id        ON payments (branch_id);
CREATE INDEX idx_payments_status           ON payments (status);
CREATE INDEX idx_payments_created_at       ON payments (created_at DESC);

-- Prevent two payment attempts being "pending" at once for the same
-- bill (e.g. customer double-tapping "pay cash").
CREATE UNIQUE INDEX one_pending_payment_per_session
    ON payments (table_session_id)
    WHERE status = 'pending';


CREATE TABLE platform_admins (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW()
);