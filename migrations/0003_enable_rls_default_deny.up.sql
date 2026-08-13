-- Supabase auto-exposes every table in the `public` schema via PostgREST
-- (REST + GraphQL) using the anon/authenticated API keys, regardless of
-- whether the app actually uses the Supabase client SDK. This app's
-- security model is entirely in the Go application layer (JWT auth, RBAC
-- middleware, tenant isolation derived server-side) — there is no
-- Postgres-native per-row policy model here, and none is needed.
--
-- This migration enables RLS on every table with ZERO permissive
-- policies for anon/authenticated. That means:
--   - PostgREST requests against these tables now return empty/denied
--     for the anon and authenticated roles — the exposure Supabase
--     flagged is fully closed.
--   - The Go backend is UNAFFECTED, because it connects using the
--     Supabase "connection string" role, which is a superuser-equivalent
--     role that bypasses RLS by default (Postgres does not apply RLS to
--     table owners/superusers unless FORCE ROW LEVEL SECURITY is set,
--     which we are not setting).
--
-- If, in the future, this app ever needs the Supabase client SDK talking
-- directly to Postgres from a browser/mobile client using the anon key,
-- real per-row policies (scoped by tenant_id, matching a JWT claim) would
-- need to be written then — this migration deliberately does not attempt
-- that, since it isn't how this app currently works.

ALTER TABLE tenants                 ENABLE ROW LEVEL SECURITY;
ALTER TABLE branches                ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff_users             ENABLE ROW LEVEL SECURITY;
ALTER TABLE tables                  ENABLE ROW LEVEL SECURITY;
ALTER TABLE table_sessions          ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_sessions       ENABLE ROW LEVEL SECURITY;
ALTER TABLE menus                   ENABLE ROW LEVEL SECURITY;
ALTER TABLE menu_categories         ENABLE ROW LEVEL SECURITY;
ALTER TABLE menu_items              ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_items         ENABLE ROW LEVEL SECURITY;
ALTER TABLE menu_item_ingredients   ENABLE ROW LEVEL SECURITY;
ALTER TABLE shared_carts            ENABLE ROW LEVEL SECURITY;
ALTER TABLE cart_participants       ENABLE ROW LEVEL SECURITY;
ALTER TABLE cart_items              ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders                  ENABLE ROW LEVEL SECURITY;
ALTER TABLE order_items             ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments                ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_admins         ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff_activities        ENABLE ROW LEVEL SECURITY;