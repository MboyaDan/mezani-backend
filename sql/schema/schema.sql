CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    plan TEXT DEFAULT 'tier1',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE branches (                         
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    location TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE staff_users (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_by UUID REFERENCES staff_users(id), 
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE tables (
    id UUID PRIMARY KEY,
    branch_id UUID NOT NULL REFERENCES branches(id),
    table_number INT NOT NULL,
    CONSTRAINT unique_table_per_branch UNIQUE (branch_id, table_number)
);

CREATE TABLE table_sessions (
    id UUID PRIMARY KEY,
    table_id UUID NOT NULL REFERENCES tables(id),
    status TEXT NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX one_active_session_per_table
ON table_sessions (table_id)
WHERE status = 'active';

CREATE TABLE customer_sessions (
    id UUID PRIMARY KEY,
    table_session_id UUID REFERENCES table_sessions(id),
    name TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- MENU SYSTEM
CREATE TABLE menus (
    id UUID PRIMARY KEY,
    branch_id UUID NOT NULL REFERENCES branches(id),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE menu_categories (
    id UUID PRIMARY KEY,
    menu_id UUID REFERENCES menus(id),
    name TEXT NOT NULL,
    display_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE menu_items (
    id UUID PRIMARY KEY,
    category_id UUID REFERENCES menu_categories(id),
    name TEXT NOT NULL,
    description TEXT,
    price NUMERIC NOT NULL,
    available BOOLEAN DEFAULT TRUE,
    sold_out BOOLEAN DEFAULT FALSE,
    is_special BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT availability_check CHECK (NOT (available = TRUE AND sold_out = TRUE))
);

-- GROUP ORDERING
CREATE TABLE shared_carts (
    id UUID PRIMARY KEY,
    table_session_id UUID REFERENCES table_sessions(id),
    created_by UUID REFERENCES customer_sessions(id),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cart_participants (
    id UUID PRIMARY KEY,
    cart_id UUID REFERENCES shared_carts(id),
    customer_session_id UUID REFERENCES customer_sessions(id),
    joined_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE cart_items (
    id UUID PRIMARY KEY,
    cart_id UUID REFERENCES shared_carts(id),
    menu_item_id UUID REFERENCES menu_items(id),
    quantity INT NOT NULL,
    added_by UUID REFERENCES customer_sessions(id),
    created_at TIMESTAMP DEFAULT NOW()
);

-- ORDERS
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    table_session_id UUID REFERENCES table_sessions(id),
    customer_session_id UUID REFERENCES customer_sessions(id),
    cart_id UUID REFERENCES shared_carts(id),  
    status TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY,
    order_id UUID REFERENCES orders(id),
    menu_item_id UUID REFERENCES menu_items(id),
    quantity INT NOT NULL
);

-- STAFF ACTIVITY
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