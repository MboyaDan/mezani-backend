-- ============================================================
-- TENANTS & BRANCHES
-- ============================================================
CREATE TABLE tenants (
    id                      UUID PRIMARY KEY,
    name                    TEXT NOT NULL,
    plan                    TEXT DEFAULT 'tier1',
    created_at              TIMESTAMP DEFAULT NOW(),
    subscription_status     TEXT NOT NULL DEFAULT 'trialing',
    subscription_expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '14 days'),
    CONSTRAINT tenants_subscription_status_check
        CHECK (subscription_status IN ('trialing', 'active', 'expired', 'cancelled'))
);

CREATE TABLE branches (
    id         UUID PRIMARY KEY,
    tenant_id  UUID NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    location   TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- STAFF
-- ============================================================
CREATE TABLE staff_users (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    branch_id     UUID REFERENCES branches(id),  
    name          TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL,
    created_by    UUID REFERENCES staff_users(id),
    created_at    TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- TABLES & SESSIONS
-- ============================================================
CREATE TABLE tables (
    id           UUID PRIMARY KEY,
    branch_id    UUID NOT NULL REFERENCES branches(id),
    table_number INT NOT NULL,
    CONSTRAINT unique_table_per_branch UNIQUE (branch_id, table_number)
);

CREATE TABLE table_sessions (
    id         UUID PRIMARY KEY,
    table_id   UUID NOT NULL REFERENCES tables(id),
    status     TEXT NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX one_active_session_per_table
    ON table_sessions (table_id)
    WHERE status = 'active';

CREATE TABLE customer_sessions (
    id               UUID PRIMARY KEY,
    table_session_id UUID REFERENCES table_sessions(id),
    name             TEXT,
    created_at       TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- MENU SYSTEM
-- ============================================================
CREATE TABLE menus (
    id         UUID PRIMARY KEY,
    branch_id  UUID NOT NULL REFERENCES branches(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE menu_categories (
    id            UUID PRIMARY KEY,
    menu_id       UUID REFERENCES menus(id),
    name          TEXT NOT NULL,
    display_order INT DEFAULT 0,
    created_at    TIMESTAMP DEFAULT NOW()
);

CREATE TABLE menu_items (
    id          UUID PRIMARY KEY,
    category_id UUID REFERENCES menu_categories(id),
    name        TEXT NOT NULL,
    description TEXT,
    price       NUMERIC NOT NULL,
    available   BOOLEAN DEFAULT TRUE,
    sold_out    BOOLEAN DEFAULT FALSE,
    is_special  BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMP DEFAULT NOW(),
    CONSTRAINT availability_check CHECK (NOT (available = TRUE AND sold_out = TRUE))
);

-- ============================================================
-- INVENTORY
-- ============================================================
CREATE TABLE inventory_items (
    id         UUID PRIMARY KEY,
    branch_id  UUID NOT NULL REFERENCES branches(id),
    name       TEXT NOT NULL,
    stock      INT NOT NULL DEFAULT 0,
    threshold  INT DEFAULT 10,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Recipe mapping: which ingredients does each menu item need?
CREATE TABLE menu_item_ingredients (
    id                  UUID PRIMARY KEY,
    menu_item_id        UUID NOT NULL REFERENCES menu_items(id),
    inventory_item_id   UUID NOT NULL REFERENCES inventory_items(id),
    quantity_required   INT NOT NULL,
    CONSTRAINT unique_item_ingredient UNIQUE (menu_item_id, inventory_item_id)
);

-- ============================================================
-- GROUP ORDERING
-- ============================================================
CREATE TABLE shared_carts (
    id               UUID PRIMARY KEY,
    table_session_id UUID REFERENCES table_sessions(id),
    created_by       UUID REFERENCES customer_sessions(id),
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cart_participants (
    id                  UUID PRIMARY KEY,
    cart_id             UUID REFERENCES shared_carts(id),
    customer_session_id UUID REFERENCES customer_sessions(id),
    joined_at           TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cart_items (
    id           UUID PRIMARY KEY,
    cart_id      UUID REFERENCES shared_carts(id),
    menu_item_id UUID REFERENCES menu_items(id),
    quantity     INT NOT NULL,
    added_by     UUID REFERENCES customer_sessions(id),
    created_at   TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- ORDERS
-- ============================================================
CREATE TABLE orders (
    id                  UUID PRIMARY KEY,
    table_session_id    UUID REFERENCES table_sessions(id),
    customer_session_id UUID REFERENCES customer_sessions(id),
    cart_id             UUID REFERENCES shared_carts(id),
    status              TEXT NOT NULL,
    created_at          TIMESTAMP DEFAULT NOW(),
    note                TEXT NOT NULL DEFAULT ''
);

CREATE TABLE order_items (
    id           UUID PRIMARY KEY,
    order_id     UUID REFERENCES orders(id),
    menu_item_id UUID REFERENCES menu_items(id),
    quantity     INT NOT NULL
);

-- ============================================================
-- PAYMENTS
-- ============================================================
CREATE TABLE payments (
    id                UUID PRIMARY KEY,
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    branch_id         UUID NOT NULL REFERENCES branches(id),
    table_session_id  UUID NOT NULL REFERENCES table_sessions(id),
    method            TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending',
    amount            NUMERIC NOT NULL,
    mpesa_receipt     TEXT,
    initiated_by      UUID REFERENCES staff_users(id),
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

CREATE UNIQUE INDEX one_pending_payment_per_session
    ON payments (table_session_id)
    WHERE status = 'pending';

-- ============================================================
-- PLANS
-- ============================================================
CREATE TABLE plans (
    id                  UUID PRIMARY KEY,
    name                TEXT UNIQUE NOT NULL,
    display_name        TEXT NOT NULL,
    price_kes           NUMERIC NOT NULL,
    billing_period_days INT NOT NULL DEFAULT 30,
    max_branches        INT NOT NULL,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMP DEFAULT NOW(),
    CONSTRAINT plans_price_check CHECK (
        price_kes > 0
        AND price_kes NOT IN ('NaN'::numeric, 'Infinity'::numeric, '-Infinity'::numeric)
    ),
    CONSTRAINT plans_billing_period_check CHECK (billing_period_days > 0),
    CONSTRAINT plans_max_branches_check CHECK (max_branches > 0)
);

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

-- ============================================================
-- PLATFORM ADMINS
-- ============================================================
CREATE TABLE platform_admins (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- STAFF ACTIVITY LOG
-- ============================================================
CREATE TABLE staff_activities (
    id          UUID PRIMARY KEY,
    staff_id    UUID NOT NULL REFERENCES staff_users(id),
    branch_id   UUID NOT NULL REFERENCES branches(id),
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    old_data    JSONB,
    new_data    JSONB,
    ip_address  INET,
    note        TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_staff_activities_staff_id   ON staff_activities (staff_id);
CREATE INDEX idx_staff_activities_entity     ON staff_activities (entity_type, entity_id);
CREATE INDEX idx_staff_activities_branch_id  ON staff_activities (branch_id);
CREATE INDEX idx_staff_activities_created_at ON staff_activities (created_at DESC);

-- ============================================================
-- INVENTORY INDEXES (query performance)
-- ============================================================
CREATE INDEX idx_inventory_items_branch_id       ON inventory_items (branch_id);
CREATE INDEX idx_menu_item_ingredients_menu_item  ON menu_item_ingredients (menu_item_id);
CREATE INDEX idx_menu_item_ingredients_inventory  ON menu_item_ingredients (inventory_item_id);

-- ============================================================
-- ADDITIONAL INDEXES (query performance)
-- ============================================================

-- Orders: most queried by table_session_id and status
CREATE INDEX idx_orders_table_session_id  ON orders (table_session_id);
CREATE INDEX idx_orders_status            ON orders (status);
CREATE INDEX idx_orders_created_at        ON orders (created_at DESC);

-- Order items: queried by order_id
CREATE INDEX idx_order_items_order_id     ON order_items (order_id);

-- Table sessions: queried by table_id + status constantly (active session lookup)
CREATE INDEX idx_table_sessions_table_id  ON table_sessions (table_id);
CREATE INDEX idx_table_sessions_status    ON table_sessions (status);
CREATE INDEX idx_table_sessions_expires_at ON table_sessions (expires_at)
    WHERE status = 'active'; -- partial index, only active sessions need expiry checks

-- Customer sessions: queried by table_session_id
CREATE INDEX idx_customer_sessions_table_session_id ON customer_sessions (table_session_id);

-- Cart items: queried by cart_id
CREATE INDEX idx_cart_items_cart_id       ON cart_items (cart_id);

-- Shared carts: queried by table_session_id
CREATE INDEX idx_shared_carts_table_session_id ON shared_carts (table_session_id);

-- Staff users: queried by email (login) and tenant_id
CREATE INDEX idx_staff_users_email        ON staff_users (email);
CREATE INDEX idx_staff_users_tenant_id    ON staff_users (tenant_id);

-- Menu: queried by branch_id
CREATE INDEX idx_menus_branch_id          ON menus (branch_id);

-- Menu categories: queried by menu_id
CREATE INDEX idx_menu_categories_menu_id  ON menu_categories (menu_id);

-- Menu items: queried by category_id
CREATE INDEX idx_menu_items_category_id   ON menu_items (category_id);

-- Payments: queried by table_session_id, tenant_id, branch_id, status
CREATE INDEX idx_payments_table_session_id ON payments (table_session_id);
CREATE INDEX idx_payments_tenant_id        ON payments (tenant_id);
CREATE INDEX idx_payments_branch_id        ON payments (branch_id);
CREATE INDEX idx_payments_status           ON payments (status);
CREATE INDEX idx_payments_created_at       ON payments (created_at DESC); 

-- Subscription payments: queried by tenant_id, status
CREATE INDEX idx_subscription_payments_tenant_id ON subscription_payments (tenant_id);
CREATE INDEX idx_subscription_payments_status    ON subscription_payments (status);