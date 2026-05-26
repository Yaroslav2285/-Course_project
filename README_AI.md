# README for AI — Service Marketplace

## Project Overview

Monorepo: Python (FastAPI, Stage 2–3) + Go escrow (Gin, Stage 4) + blockchain simulator (Python/FastAPI, Stage 5).

**Branch:** `step_4` — all work is here.
**Server:** `http://localhost:8000` (Python API), `http://localhost:8081` (Go escrow).
**DB:** PostgreSQL via Docker Compose; SQLite works for local dev without Docker.

---

## Current State (after Stage 4)

### Stage 2 — DB / Migrations
- SQLAlchemy async models (`users`, `services`, `orders`) with `NUMERIC(19,4)` for prices
- Alembic async migration (`fff36bd858b0_init.py`) — idempotent, creates tables + FK + indexes
- `Base.metadata.create_all` in lifespan for dev simplicity alongside Alembic for prod
- `scripts/seed_db.py` — registers client + provider, creates/publishes service, creates order via API

### Stage 3 — Python API (FastAPI)
- JWT auth (access + refresh tokens) with bcrypt
- CRUD for users, services, orders with pagination
- Web UI (Jinja2 + Vanilla JS): catalog, dashboard, escrow status page
- 23 pytest tests pass (92% coverage), `ruff check .` clean
- Dockerized with curl healthcheck, non-root user

### Stage 4 — Go Escrow Microservice (Gin)
- State machine: `created → funded → released` (also `cancelled` / `disputed`)
- Idempotency via `X-Idempotency-Key` (in-memory map with TTL + goroutine cleanup)
- Token-bucket rate limiter on `/fund` endpoint
- Custom SQL migration (no ORM) — `001_initial.sql` with `escrow_accounts`, `transactions`, `disputes`
- Auto-migration on startup
- Zap JSON structured logging
- Graceful shutdown (signal handling)
- 47 table-driven tests pass, `gosec` — 0 issues
- Multistage Docker build (distroless non-root image)

### Docker Compose
- 3 containers: `postgres` (15-alpine), `python-api` (port 8000), `go-escrow` (port 8081)
- `.dockerignore` for python-api (excludes `.env` to prevent SQLite override in container)
- Healthchecks on all services

---

## Quick Start

### Docker (PostgreSQL + all services)
```powershell
docker compose up -d --build
# Verify:
docker compose ps                    # all 3 containers healthy
curl http://localhost:8000/health    # Python API
curl http://localhost:8081/health    # Go escrow
```

### Local dev (SQLite, no Docker)
```powershell
cd services/python-api
pip install -r requirements.txt
uvicorn main:app --host 0.0.0.0 --port 8000
```

### Seed demo data
```powershell
python scripts/seed_db.py http://localhost:8000
# Credentials: client@demo.com / Demo123!, provider@demo.com / Demo123!
```

---

## Architecture

### Python API (`services/python-api/`)

```
services/python-api/
├── main.py              # App factory: lifespan, middleware, error handlers, router mounting
├── core/
│   ├── config.py        # Pydantic Settings (env vars, defaults to SQLite for local)
│   ├── db.py            # Async SQLAlchemy engine + session factory + get_db()
│   ├── security.py      # bcrypt password hashing, JWT create/decode
│   ├── exceptions.py    # Custom HTTP exceptions + JSON error handlers
│   ├── deps.py          # FastAPI dependencies: get_current_user, pagination_params
│   └── responses.py     # success_response() helper — unified JSON envelope
├── models/              # SQLAlchemy ORM models
│   ├── base.py          # DeclarativeBase
│   ├── users.py         # User (id, email, hashed_password, role)
│   ├── services.py      # Service (id, provider_id, title, description, price, status)
│   └── orders.py        # Order (id, service_id, buyer_id, seller_id, amount, status)
├── schemas/             # Pydantic V2 schemas
│   ├── common.py        # Shared types/pagination
│   ├── users.py         # UserCreate, UserLogin, UserRead, TokenResponse, TokenRefresh
│   ├── services.py      # ServiceCreate, ServiceUpdate, ServiceRead
│   └── orders.py        # OrderCreate, OrderStatusUpdate, OrderRead
├── repositories/        # Data access layer (generic CRUD base + per-entity)
│   ├── base.py          # RepositoryBase[ModelT] — create, get, list, update, delete
│   ├── users.py         # UserRepository — get_by_email, create_user
│   ├── services.py      # ServiceRepository — list_published, create_service, update_service
│   └── orders.py        # OrderRepository — create_order, update_status, list_by_buyer/seller
├── api/v1/              # FastAPI routers (all under /v1)
│   ├── auth.py          # POST /register, /login, /refresh
│   ├── users.py         # GET /me, PUT /me
│   ├── services.py      # GET / (published), GET /my, GET /{id}, POST /, PUT /{id}, DELETE /{id}
│   ├── orders.py        # GET /, GET /sold, GET /{id}, POST /, PATCH /{id}/status
│   ├── ui.py            # Web UI routes + mount_static() helper
│   └── __init__.py      # Aggregates all routers under prefix="/v1"
├── templates/           # Jinja2 templates
│   ├── base.html        # Layout: navbar, alert container, main, app.js, CSS
│   ├── index.html       # Catalog — fetches published services, pagination, order button
│   ├── login.html       # Register/Login form (toggle, client-side validation)
│   ├── dashboard.html   # My services, orders (bought + sold), create-service modal
│   └── escrow_status.html  # Escrow flow visualization + status management buttons
├── static/
│   ├── css/style.css    # 575 lines: custom properties, grid, cards, badges, modal, escrow flow
│   └── js/app.js        # 260 lines: fetch client, auth, services CRUD, orders, escrow render, navbar
├── tests/               # Async pytest suite
│   ├── conftest.py      # Forces SQLite env var, registers now() on test engine
│   ├── test_auth.py
│   ├── test_users.py
│   ├── test_services.py
│   └── test_orders.py
├── alembic/             # Alembic async migrations
│   ├── versions/fff36bd858b0_init.py
│   ├── env.py
│   └── script.py.mako
├── Dockerfile           # Python 3.12-slim, curl healthcheck, non-root user
└── requirements.txt     # fastapi, uvicorn, sqlalchemy, alembic, asyncpg, etc.
```

### Go Escrow (`services/go-escrow/`)

```
services/go-escrow/
├── main.go                     # Entry point: config load, DB init, router setup, graceful shutdown
├── internal/
│   ├── api/
│   │   ├── handler.go          # POST /v1/escrow, /fund, /release, /dispute, GET /{id}, /health
│   │   ├── handler_test.go     # 25+ HTTP tests with httptest
│   │   ├── middleware.go       # Token-bucket rate limiter (1 req/100ms per client)
│   │   ├── idempotency.go      # In-memory idempotency store (sync.Map + TTL + cleanup goroutine)
│   │   └── router.go           # Gin router setup
│   ├── config/
│   │   └── config.go           # Env-based config (DB_DSN, PORT, RATE_LIMIT_RATE/BURST)
│   ├── db/
│   │   └── db.go               # database/sql + pgx pool, auto-migration on startup
│   ├── domain/
│   │   ├── escrow.go           # EscrowAccount, Transaction, Dispute structs + status constants
│   │   └── escrow_test.go      # State-machine transition tests (22+)
│   ├── repository/
│   │   └── escrow_repo.go      # CRUD on escrow_accounts, transactions, disputes via SQL
│   └── service/
│       ├── escrow_service.go   # Business logic: create, fund, release, dispute, cancel
│       └── escrow_service_test.go  # Service-level tests with mock repo
├── Dockerfile                  # Multistage: golang:1.23-alpine → gcr.io/distroless/static-debian12
└── Makefile                    # build, test, lint (golangci-lint), sec (gosec)
```

### DB Tables (PostgreSQL)

**`users`** — `id (UUID)`, `email`, `hashed_password`, `role`, `created_at`, `updated_at`

**`services`** — `id (UUID)`, `provider_id → users(id)`, `title`, `description`, `price (NUMERIC(19,4))`, `status`, `created_at`, `updated_at`

**`orders`** — `id (UUID)`, `service_id → services(id)`, `buyer_id → users(id)`, `seller_id → users(id)`, `amount (NUMERIC(19,4))`, `status`, `notes`, `created_at`, `updated_at`

**`escrow_accounts`** — `id (UUID)`, `order_id`, `balance (NUMERIC)`, `status`, `created_at`, `updated_at`

**`transactions`** — `id (UUID)`, `escrow_account_id → escrow_accounts(id)`, `order_id`, `amount`, `type`, `status`, `created_at`

**`disputes`** — `id (UUID)`, `escrow_account_id → escrow_accounts(id)`, `order_id`, `reason`, `status`, `created_at`, `updated_at`

> Note: Go escrow tables have no FK to Python API `orders` — cross-service validation is handled at the API level.

---

## API Reference

### Python API (port 8000)

Unified response envelope:
- **Success:** `{"data": ..., "total": N, "limit": N, "offset": N}`
- **Error:** `{"errors": [{"code": "...", "detail": "..."}]}`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/v1/auth/register` | No | Register user. Body: `{email, password, role}` (client/provider/admin). Returns tokens + user |
| POST | `/v1/auth/login` | No | Login. Body: `{email, password}`. Returns tokens + user |
| POST | `/v1/auth/refresh` | No | Refresh tokens. Body: `{refresh_token}` |
| GET | `/v1/users/me` | Yes | Current user info |
| PUT | `/v1/users/me` | Yes | Update current user (NOT PATCH — full PUT) |
| GET | `/v1/services/` | No | Published services. Query: `limit`, `offset`, `status` (defaults to published) |
| GET | `/v1/services/my` | Yes | Current provider's services (all statuses) |
| GET | `/v1/services/{id}` | No | Single service by ID |
| POST | `/v1/services/` | Yes | Create service (default status=draft). Body: `{title, description?, price}` |
| PUT | `/v1/services/{id}` | Yes | Update service (own only). Body: `{title?, description?, price?, status?}` |
| DELETE | `/v1/services/{id}` | Yes | Delete service (own only) |
| GET | `/v1/orders/` | Yes | Current user's bought orders. Query: `limit`, `offset`, `status` |
| GET | `/v1/orders/sold` | Yes | Current user's sold orders (as provider). Query: `limit`, `offset`, `status` |
| GET | `/v1/orders/{id}` | Yes | Single order |
| POST | `/v1/orders/` | Yes | Create order. Body: `{service_id, seller_id, amount, notes?}` |
| PATCH | `/v1/orders/{id}/status` | Yes | Update order status. Body: `{status}` (pending/funded/released/cancelled/disputed) |

**Auth:** `Authorization: Bearer <access_token>` (access: 30 min, refresh: 7 days)

**Important:** All endpoints require trailing slash (`/v1/services/` not `/v1/services/`).

### Go Escrow API (port 8081)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | No | Health check |
| POST | `/v1/escrow` | No | Create escrow account. Body: `{order_id, amount}` |
| POST | `/v1/escrow/{id}/fund` | No | Fund escrow (rate-limited: 1 req/100ms). Body: `{amount}` |
| POST | `/v1/escrow/{id}/release` | No | Release funds to provider |
| POST | `/v1/escrow/{id}/dispute` | No | Open dispute. Body: `{reason}` |
| GET | `/v1/escrow/{id}` | No | Get escrow account details + transactions |

**Headers:** `Content-Type: application/json`, `X-Idempotency-Key` (optional, any POST)

**Status codes:** 201 (created), 200 (success), 400 (bad request), 404 (not found), 409 (invalid transition), 429 (rate-limited)

**Escrow state machine:**
```
created → funded → released
    ↓        ↓
cancelled  disputed
```

---

## Web UI

### Pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | Catalog (`index.html`) | Fetches published services in grid, pagination, order button |
| `/login` | Auth (`login.html`) | Register/Login toggle, form validation |
| `/dashboard` | Dashboard (`dashboard.html`) | My services (create/delete), my orders, my sales |
| `/escrow/{order_id}` | Escrow Status (`escrow_status.html`) | Visual flow (pending → funded → released), status management |

### JS Client (`static/js/app.js`)

- `apiFetch(path, options)` — base fetch with auth header, JSON parse, error extraction
- Auth helpers: `isLoggedIn()`, `getToken()`, `setToken()`, `clearTokens()`
- CRUD: `fetchServices()`, `fetchMyServices()`, `createService()`, `updateService()`, `deleteService()`
- Orders: `fetchOrders()`, `fetchSoldOrders()`, `createOrder()`, `updateOrderStatus()`, `getOrder()`
- Render: `renderServiceCard()`, `renderOrderRow()`, `renderEscrowStatus()`, `renderBadge()`
- `showAlert()`, `clearAlerts()`, `escapeHtml()`, `renderNavbar()`

---

## Data Model

### User Roles: `client | provider | admin`

### Service Statuses: `draft → published → archived`
- Default `draft` on creation
- Only `published` appear in catalog (default filter)
- Provider updates via `PUT /v1/services/{id}`

### Order Statuses (Escrow Flow):
```
pending → funded → released
    ↓         ↓
cancelled   disputed
```

### Escrow Account Statuses:
```
created → funded → released
    ↓        ↓
cancelled  disputed
```

---

## Key Fixes Applied

### Stage 2 (DB/Makefile/Seed)
| Problem | Fix | File |
|---------|-----|------|
| Duplicate `own:` target in Makefile | Removed duplicate | `Makefile` |
| No test target | Added `test-python` and `test-go` | `Makefile` |
| No demo data seeder | Created `seed_db.py` | `scripts/seed_db.py` |
| Docker healthcheck failed (no curl) | `apt-get install curl` | `Dockerfile` |
| `.env` overrode docker-compose `DB_URL` | Created `.dockerignore` excluding `.env` | `.dockerignore` |

### Stage 3 (Python API)
| Problem | Fix | File(s) |
|---------|-----|---------|
| 500 error on `/` | TemplateResponse argument order (request first) | `api/v1/ui.py` |
| Static files 404 | Changed to `url_for('static', path='...')` | `templates/base.html` |
| get_db() didn't persist | Added `session.commit()` on success | `core/db.py` |
| Unhandled exceptions silent | `logging.getLogger().exception()` | `core/exceptions.py` |
| PostgreSQL dependency for local dev | Default `DB_URL` = `sqlite+aiosqlite` | `core/config.py` |
| Tests failed on SQLite | Force SQLite env var, register `now()` | `tests/conftest.py`, `core/db.py` |
| Tables not created | Auto-create in `lifespan` (all DB engines) | `main.py` |
| pydantic email validation error | Added `email-validator` to requirements | `requirements.txt` |

### Stage 4 (Go Escrow)
| Problem | Fix | File(s) |
|---------|-----|---------|
| FK to `orders` table (cross-service coupling) | Removed FK constraints | `001_initial.sql` |
| Non-deterministic test failure (rate-limit) | Isolated rate-limiter per test | `handler_test.go` |
| Race condition in idempotency cleanup | Added `sync.Map` + cleanup goroutine with done channel | `idempotency.go` |

---

## Running Tests

### Python API
```powershell
cd services/python-api
pytest -v --cov --cov-report=term
ruff check .
```

### Go Escrow
```powershell
cd services/go-escrow
go test ./... -v -cover
gosec ./...                  # security scan
golangci-lint run ./...      # linter
```

### All at once (from repo root)
```powershell
make test
```

---

## Known Issues / Next Steps

- **Stage 5 (done):** Blockchain simulator service (Python/FastAPI) — SHA-256 audit trail for escrow status changes
- Integration tests across Python + Go + Blockchain-sim not yet written
- UI lacks: password reset, profile editing, admin panel, service search/filter
- Orders don't actually interact with external escrow system (order status updated via PATCH, not through go-escrow)
- Push to remote when ready
