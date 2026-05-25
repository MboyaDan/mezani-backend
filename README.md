# Mezzani Backend

> Production-grade REST API for the Mezzani Restaurant OS — a multi-tenant QR ordering and kitchen management platform built for African restaurants.

---

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Database](#database)
- [API Reference](#api-reference)
- [Authentication & Authorization](#authentication--authorization)
- [Real-Time (WebSocket)](#real-time-websocket)
- [Background Workers](#background-workers)
- [Notifications](#notifications)
- [Caching](#caching)
- [Rate Limiting](#rate-limiting)
- [Logging & Alerts](#logging--alerts)
- [Running Tests](#running-tests)
- [Deployment](#deployment)
- [Contributing](#contributing)

---

## Overview

Mezzani is a multi-tenant SaaS platform that enables restaurants to:

- Accept orders via QR codes — no app download required
- Manage kitchen operations with a real-time Kitchen Display System (KDS)
- Track inventory, staff, menus, and analytics across multiple branches
- Support multiple roles: owner, manager, waiter, kitchen, cashier

Each restaurant is a **tenant** with fully isolated data. Branches, menus, staff, orders and analytics are all scoped per tenant.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25+ |
| Web Framework | Gin |
| Database | PostgreSQL 15 |
| Query Layer | sqlc (type-safe, no ORM) |
| DB Driver | pgx/v5 + pgxpool |
| Migrations | golang-migrate |
| Cache | Redis 7 |
| Real-time | WebSocket (gorilla/websocket) + Redis Pub/Sub |
| Auth | JWT (HMAC-SHA256) + refresh token rotation |
| Password Hashing | bcrypt (cost 12) |
| Email | Resend API |
| WhatsApp | Meta Cloud API |
| Alerts | Telegram Bot API |
| Containerisation | Docker + Docker Compose |
| Logging | Go slog (structured) |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Gin Router                           │
│   Public Routes          Protected Routes (JWT required)    │
│   /auth/*                /dashboard/*, /kitchen/*, etc.     │
└───────────────┬─────────────────────────┬───────────────────┘
                │                         │
        ┌───────▼──────┐         ┌────────▼────────┐
        │   Handlers   │         │   Middleware     │
        │  (HTTP layer)│         │  Auth + RBAC +   │
        └───────┬──────┘         │  Rate Limit      │
                │                └─────────────────┘
        ┌───────▼──────┐
        │   Services   │  ← Business logic
        └───────┬──────┘
                │
        ┌───────▼──────┐         ┌──────────────┐
        │    sqlc      │─────────│  PostgreSQL   │
        │  (Queries)   │         └──────────────┘
        └───────┬──────┘
                │
        ┌───────▼──────┐         ┌──────────────┐
        │  Event Bus   │─────────│    Redis      │
        │ (Pub/Sub)    │         │  Cache + Jobs │
        └───────┬──────┘         └──────────────┘
                │
        ┌───────▼──────────────────────────┐
        │         Workers                  │
        │  Kitchen Worker  Session Worker  │
        │  WhatsApp Worker                 │
        └──────────────────────────────────┘
```

### Request Lifecycle

```
HTTP Request
  → CORS Middleware
  → Rate Limit Middleware
  → Auth Middleware (validates JWT, injects claims into context)
  → RequirePermission Middleware (checks role permissions)
  → Handler (binds + validates request body)
  → Service (business logic, DB queries, event publishing)
  → Response
```

---

## Project Structure

```
mezzani_backend/
├── cmd/
│   └── server/           # Application entrypoint (main.go)
├── docker/
│   └── Dockerfile        # Multi-stage Go build
├── internal/
│   ├── auth/             # JWT Claims struct
│   ├── common/           # Shared error types
│   ├── config/           # Config loader (viper)
│   ├── database/
│   │   ├── redis.go      # Redis client factory
│   │   └── sqlc/         # sqlc-generated type-safe query code
│   ├── domain/           # Business domain types
│   │   ├── roles.go      # Role constants + permission maps
│   │   └── order_status.go # Order state machine
│   ├── handler/          # HTTP handlers (one file per resource)
│   ├── middleware/        # Gin middleware
│   │   ├── auth.go       # JWT validation
│   │   ├── permission.go # RBAC enforcement
│   │   ├── rate_limit.go # Per-IP rate limiting
│   │   ├── logger.go     # Structured request logger
│   │   └── recovery.go   # Panic recovery + Telegram alert
│   ├── notifications/    # External notifications
│   │   ├── eventbus.go   # Redis Pub/Sub event bus
│   │   ├── hub.go        # WebSocket hub
│   │   ├── kds_events.go # Kitchen Display event types
│   │   ├── kitchen_worker.go
│   │   ├── whatsapp.go
│   │   ├── email.go      # Resend integration
│   │   └── alerts.go     # Telegram alerts
│   ├── router/           # Route registration
│   ├── service/          # Business logic layer
│   └── workers/          # Background workers
│       └── session_expiry.go
├── migrations/           # SQL migration files
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── sql/
│   ├── schema/           # Database schema (source of truth)
│   └── queries/          # sqlc query definitions
├── docker-compose.yml
├── sqlc.yaml
├── go.mod
└── go.sum
```

---

## Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

### 1. Clone the repository

```bash
git clone https://github.com/yourname/mezzani_backend.git
cd mezzani_backend
```

### 2. Set up environment variables

```bash
cp .env.example .env
# Edit .env with your values
```

### 3. Start all services

```bash
docker compose up --build
```

This starts:
- PostgreSQL 15 on port `5432`
- Redis 7 on port `6379`
- Mezzani API on port `8080`

Migrations run automatically on startup via `golang-migrate`.

### 4. Verify

```bash
curl http://localhost:8080/health
# → {"status":"ok","time":"..."}
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | ✅ | PostgreSQL connection string |
| `REDIS_URL` | ✅ | Redis connection URL (`redis://redis:6379`) |
| `JWT_SECRET` | ✅ | Secret key for HMAC-SHA256 JWT signing (min 32 chars) |
| `PORT` | ✅ | HTTP server port (default: `8080`) |
| `APP_ENV` | ✅ | `development` or `production` |
| `ALLOWED_ORIGINS` | ✅ | Comma-separated CORS origins |
| `WHATSAPP_TOKEN` | ✅ | Meta Cloud API access token |
| `WHATSAPP_PHONE_ID` | ✅ | WhatsApp phone number ID |
| `RESEND_API_KEY` | ✅ | Resend API key for transactional email |
| `RESEND_FROM_EMAIL` | ✅ | Sender email address |
| `FRONTEND_URL` | ✅ | Frontend URL (used in password reset links) |
| `TELEGRAM_BOT_TOKEN` | ✅ | Telegram bot token for alerts |
| `TELEGRAM_CHAT_ID` | ✅ | Telegram chat ID for alert delivery |

### Example `.env`

```env
APP_ENV=development
PORT=8080

DATABASE_URL=postgres://postgres:password@postgres:5432/mezzani?sslmode=disable
REDIS_URL=redis://redis:6379

JWT_SECRET=your-super-secret-key-minimum-32-chars

ALLOWED_ORIGINS=http://localhost:3000,https://yourdomain.com

WHATSAPP_TOKEN=your_meta_token
WHATSAPP_PHONE_ID=your_phone_id

RESEND_API_KEY=re_xxxxxxxxx
RESEND_FROM_EMAIL=noreply@yourdomain.com
FRONTEND_URL=http://localhost:3000

TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id
```

---

## Database

### Schema Overview

```
tenants              — Root entity. Each restaurant is one tenant.
  └── branches       — Physical locations (Westlands, CBD, etc.)
       ├── tables    — Physical tables per branch
       ├── menus     — Menu per branch
       └── staff_users — Staff assigned to branch

table_sessions       — Active QR session per table (expires)
  └── customer_sessions — Customers who joined the session
       └── shared_carts  — Group cart per session
            └── cart_items

orders               — Submitted orders from a cart
  └── order_items

inventory_items      — Stock tracking per branch
menu_item_ingredients — Recipe: which inventory items a menu item needs
staff_activities     — Audit log for all staff actions
```

### Migrations

Migrations use `golang-migrate` and run automatically on app startup:

```go
// cmd/server/main.go
runMigrations(cfg.DatabaseURL)
```

To run manually:

```bash
# Apply
migrate -path migrations -database "$DATABASE_URL" up

# Rollback one step
migrate -path migrations -database "$DATABASE_URL" down 1
```

### Regenerating sqlc

After modifying any file in `sql/queries/`:

```bash
sqlc generate
```

> ⚠️ Never edit files in `internal/database/sqlc/` directly — they are auto-generated.

---

## API Reference

### Base URL

```
http://localhost:8080/api
```

### Public Endpoints (no auth required)

#### Auth

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/auth/register-owner` | Register a new restaurant owner + tenant |
| `POST` | `/auth/login` | Login, returns `access_token` + `refresh_token` |
| `POST` | `/auth/refresh` | Refresh access token using refresh token |
| `POST` | `/auth/logout` | Revoke refresh token |
| `POST` | `/auth/forgot-password` | Send password reset email |
| `POST` | `/auth/reset-password` | Reset password using token from email |

#### Customer Flow (QR scan — no JWT)

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/customer/join` | Join a table session by name |
| `GET` | `/customer/:id` | Get customer session |
| `GET` | `/customer/table/:table_session_id` | List customers at table |
| `POST` | `/cart/create` | Create a shared cart |
| `POST` | `/cart/join` | Join an existing cart |
| `POST` | `/cart/add-item` | Add item to cart |
| `POST` | `/orders/submit` | Submit cart as order |

#### Menu Viewing

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/session/:session_id/menu` | Get full menu for a table session |
| `GET` | `/session/:session_id/info` | Get table number + branch info |
| `GET` | `/menus/:menu_id/full` | Get full nested menu (categories + items) |
| `GET` | `/menus/branch/:branch_id` | List menus for a branch |
| `GET` | `/menus/:menu_id/categories` | List menu categories |
| `GET` | `/categories/:category_id/items` | List items in a category |

---

### Protected Endpoints (JWT required)

All protected endpoints require:
```
Authorization: Bearer <access_token>
```

#### Table Sessions

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/table-session/start` | any staff | Start a table session |
| `POST` | `/table-session/:id/close` | any staff | Close a session |
| `POST` | `/table-session/heartbeat` | any staff | Extend session by 30 min |

#### Orders (Staff)

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `GET` | `/orders/recent` | any staff | Recent orders for branch |
| `PATCH` | `/orders/:id/status` | `update_order_status` | Update order status |

#### Kitchen

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `PATCH` | `/kitchen/orders/:id/status` | `view_kitchen_display` | Update order status from KDS |

#### Billing

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/billing/close` | `close_bill` | Close bill for table session |

#### Menu Management

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/menu/` | `manage_menu` | Create menu |
| `POST` | `/menu/categories` | `manage_menu` | Create category |
| `POST` | `/menu/items` | `manage_menu` | Create menu item |
| `PATCH` | `/menu/items/:id/price` | `manage_menu` | Update item price |
| `PATCH` | `/menu/items/:id/sold-out` | `manage_menu` | Mark item sold out |
| `PATCH` | `/menu/items/:id/available` | `manage_menu` | Mark item available |
| `PATCH` | `/menu/items/:id/special` | `manage_menu` | Toggle daily special |
| `DELETE` | `/menu/items/:id` | `manage_menu` | Delete menu item |

#### Inventory

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/branches/:branch_id/inventory/` | `manage_inventory` | Create inventory item |
| `GET` | `/branches/:branch_id/inventory/` | `manage_inventory` | List inventory items |
| `GET` | `/branches/:branch_id/inventory/:item_id` | `manage_inventory` | Get single item |
| `PATCH` | `/branches/:branch_id/inventory/:item_id/stock` | `manage_inventory` | Update stock level |
| `PATCH` | `/branches/:branch_id/inventory/low-stock` | `manage_inventory` | Get low stock alerts |
| `DELETE` | `/branches/:branch_id/inventory/:item_id` | `manage_inventory` | Delete inventory item |

#### Staff Management

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/staff/create` | `manage_staff` | Create staff member |
| `GET` | `/staff/list` | `manage_staff` | List staff for branch |
| `DELETE` | `/staff/:id` | `manage_staff` | Delete staff member |

#### Branches & Tables (Owner)

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `POST` | `/owner/branches` | `manage_branches` | Create branch |
| `GET` | `/owner/branches` | `manage_branches` | List owner's branches |
| `DELETE` | `/owner/branches/:id` | `manage_branches` | Delete branch |
| `POST` | `/owner/branches/:id/tables` | `manage_branches` | Create table |
| `GET` | `/tables` | any staff | List tables with session status |

#### Analytics

| Method | Endpoint | Permission | Description |
|---|---|---|---|
| `GET` | `/reports/analytics/dashboard` | `view_reports` | Dashboard analytics |

---

### Request / Response Examples

#### Register Owner

```bash
POST /api/auth/register-owner
Content-Type: application/json

{
  "restaurant_name": "Mama Njeri Kitchen",
  "email": "mama@njeri.com",
  "password": "securepassword"
}
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "a3f9c2e1b8d4..."
}
```

#### Submit Order

```bash
POST /api/orders/submit
Content-Type: application/json

{
  "table_session_id": "883b864d-c037-49d1-9032-d6d7757a3105",
  "customer_session_id": "1ffe5d16-23cb-4fb3-8e6c-c389babbeaf2",
  "cart_id": "51f25481-dce0-4e45-8ba6-d2a121eaaa44"
}
```

```json
{
  "ID": "e05db1b3-c54b-42bb-94f7-0a6b12ac7243",
  "TableSessionID": "883b864d-c037-49d1-9032-d6d7757a3105",
  "CustomerSessionID": "1ffe5d16-23cb-4fb3-8e6c-c389babbeaf2",
  "CartID": "51f25481-dce0-4e45-8ba6-d2a121eaaa44",
  "Status": "pending",
  "CreatedAt": "2026-03-25T08:39:15.140781Z"
}
```

#### Update Order Status (Kitchen)

```bash
PATCH /api/kitchen/orders/:id/status
Authorization: Bearer <token>
Content-Type: application/json

{
  "order_id": "e05db1b3-c54b-42bb-94f7-0a6b12ac7243",
  "status": "preparing"
}
```

Valid status transitions:
```
pending → accepted → preparing → ready → served → closed
```

---

## Authentication & Authorization

### JWT Structure

Access tokens are signed with HMAC-SHA256 and contain:

```json
{
  "uid": "user-uuid",
  "tid": "tenant-uuid",
  "tname": "Mama Njeri Kitchen",
  "bid": "branch-uuid",
  "role": "owner",
  "tca": 1711234567,
  "plan": "tier1",
  "exp": 1711235467,
  "iat": 1711234567,
  "iss": "mezzani-api"
}
```

| Claim | Description |
|---|---|
| `uid` | Staff user ID |
| `tid` | Tenant (restaurant) ID — used for data isolation |
| `tname` | Restaurant name |
| `bid` | Branch ID (empty for owners) |
| `role` | Staff role |
| `tca` | Tenant created_at (unix) — used for trial calculation |
| `plan` | Subscription plan (`tier1`, `tier2`, `tier3`) |

### Token Lifecycle

```
Login
  → access_token (15 minutes, stateless)
  → refresh_token (7 days, stored in Redis as SHA256 hash)

On 401 (expired access token)
  → POST /auth/refresh with refresh_token
  → Old refresh token deleted (rotation)
  → New token pair issued

On logout
  → POST /auth/logout with refresh_token
  → Redis key deleted immediately
```

### Role Permissions

| Permission | Owner | Manager | Waiter | Kitchen | Cashier |
|---|---|---|---|---|---|
| `manage_branches` | ✅ | | | | |
| `manage_staff` | ✅ | | | | |
| `manage_menu` | ✅ | ✅ | | | |
| `manage_inventory` | ✅ | ✅ | | | |
| `view_reports` | ✅ | ✅ | | | |
| `manage_tables` | ✅ | ✅ | | | |
| `create_orders` | ✅ | | ✅ | | |
| `update_order_status` | ✅ | | | ✅ | |
| `view_kitchen_display` | ✅ | | | ✅ | |
| `close_bill` | ✅ | | | | ✅ |

### Tenant Isolation

Every database query that touches branch-level data is scoped by `tenant_id` extracted from the JWT. The `TenantBranchGuard` middleware validates that the `branch_id` in the URL belongs to the requesting tenant before the handler runs.

---

## Real-Time (WebSocket)

### Kitchen Display System

The KDS uses a WebSocket hub backed by Redis Pub/Sub for multi-instance support.

**Connection:**
```
ws://localhost:8080/ws/kitchen
```

No auth required on the WebSocket connection itself — the kitchen display is assumed to be on a trusted internal network. Auth can be added via query param if needed.

**Message types received by frontend:**

```json
// New order submitted
{
  "type": "new_order",
  "order_id": "uuid",
  "table_id": "session-uuid",
  "table_number": 4,
  "items": [
    { "name": "Chapati + Beans", "quantity": 2 },
    { "name": "Masala Chai", "quantity": 1 }
  ],
  "note": "Extra chili"
}

// Order status updated
{
  "type": "order_updated",
  "order_id": "uuid",
  "status": "preparing"
}
```

**Flow:**
```
Customer submits order
  → OrderService.SubmitCart()
  → EventBus.Publish("orders.new", event)
  → Redis Pub/Sub
  → KitchenWorker receives message
  → Hub.Broadcast to all connected WebSocket clients
  → Kitchen screen updates in real time
```

---

## Background Workers

### Session Expiry Worker

Runs every 60 seconds. Finds all active table sessions where `expires_at < NOW()` and closes them.

```go
// internal/workers/session_expiry.go
go workers.StartSessionExpiryWorker(queries)
```

### Kitchen Worker

Subscribes to `orders.new` and `orders.status` Redis channels. Forwards events to the WebSocket hub for real-time kitchen display updates.

```go
go notifications.StartKitchenWorker(eventBus, hub)
```

### WhatsApp Worker

Subscribes to `orders.new`. Sends a WhatsApp notification to the restaurant's registered phone number when a new order arrives.

```go
go notifications.StartWhatsAppWorker(eventBus, whatsapp)
```

---

## Notifications

### Email (Resend)

Used for password reset flows. Sends a branded HTML email with a time-limited reset link.

```go
emailSender := notifications.NewEmailSender(cfg.ResendAPIKey, cfg.ResendFromEmail)
```

Reset tokens are stored in Redis with a 15-minute TTL under the key `password_reset:<token>`.

### WhatsApp (Meta Cloud API)

Sends order notifications to restaurant owners via WhatsApp when new orders arrive.

```go
whatsapp := notifications.NewWhatsAppSender(cfg.WhatsappToken, cfg.WhatsappPhoneID)
```

### Telegram Alerts

Used for production monitoring. The recovery middleware sends a Telegram message on every panic/500 error with the stack trace and request details.

```go
alertService := notifications.NewAlertService(cfg.TelegramBotToken, cfg.TelegramChatID)
```

Alert levels:
- 🔴 **Critical** — server panics, 500 errors
- 🟡 **Warning** — non-critical failures
- 🟢 **Info** — server startup, deployments

---

## Caching

Menu data is cached in Redis to reduce database load. Cache is invalidated on write operations.

| Cache Key Pattern | TTL | Invalidated On |
|---|---|---|
| `menu:full:<menu_id>` | 10 min | Any menu item change |
| `menu:branch:<branch_id>` | 30 min | Menu created/updated |
| `menu:categories:<menu_id>` | 10 min | Category created |
| `menu:items:<category_id>` | 10 min | Item created/updated |

Cache operations use `cache.DeleteByPattern("menu:*")` on writes that affect multiple keys.

---

## Rate Limiting

Per-IP rate limiting using in-memory sliding window counters.

| Tier | Limit | Applied To |
|---|---|---|
| Strict | 10 req/min | `/auth/login`, `/auth/register-owner`, `/auth/forgot-password`, `/auth/reset-password` |
| Moderate | 60 req/min | Customer flow endpoints (join, cart, order) |
| Relaxed | 120 req/min | Menu viewing, all protected routes |

---

## Logging & Alerts

### Request Logging

Every request is logged with method, path, status code, latency and client IP. Color-coded in development:
- 🟢 Green — 2xx
- 🟡 Yellow — 4xx
- 🔴 Red — 5xx

### Structured Logging

Services use `slog` for structured contextual logging:

```go
s.Logger.ErrorContext(ctx, "failed to create tenant", "error", err, "email", email)
```

### Panic Recovery

The `RecoveryWithAlerts` middleware catches all panics, returns a clean `500` to the client, and sends a Telegram alert with the stack trace.

---

## Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run a specific package
go test ./internal/service/...
```

---

## Deployment

### Docker Compose (Development)

```bash
# Start all services
docker compose up --build

# Stop and remove volumes (fresh start)
docker compose down -v

# View logs
docker compose logs -f backend
```

### Production Checklist

Before deploying to production:

- [ ] Set `APP_ENV=production`
- [ ] Use a strong `JWT_SECRET` (min 32 random characters)
- [ ] Set `ALLOWED_ORIGINS` to your actual frontend domain
- [ ] Use a managed PostgreSQL instance (not Docker in prod)
- [ ] Use a managed Redis instance (not Docker in prod)
- [ ] Enable PostgreSQL SSL (`sslmode=require` in `DATABASE_URL`)
- [ ] Set up Telegram alerts with a dedicated bot
- [ ] Configure a custom domain for the Resend sender email
- [ ] Set up a reverse proxy (Nginx/Caddy) with SSL termination
- [ ] Enable database backups

### Health Check

```bash
GET /health
```

```json
{
  "status": "ok",
  "time": "2026-04-21T10:00:00Z"
}
```

### Recommended Production Stack

| Component | Service |
|---|---|
| Backend hosting | Railway / Render / Fly.io |
| PostgreSQL | Supabase / Railway Postgres / Neon |
| Redis | Upstash / Railway Redis |
| Domain + SSL | Cloudflare |
| Monitoring | Telegram alerts (built-in) |

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Run `sqlc generate` if you modified any SQL queries
6. Commit with a descriptive message
7. Open a pull request

### Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Keep handlers thin — business logic belongs in services
- Never edit `internal/database/sqlc/` files directly
- Use `common.Err*` error types for consistent API error responses
- Log errors with context using `slog.ErrorContext`

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

Built with ☕ in Nairobi, Kenya.
