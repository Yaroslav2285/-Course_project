# README for AI — Service Marketplace

## Project Overview

Monorepo: Python (FastAPI) backend + Go escrow service (scaffolded) + blockchain simulator (planned).

**Branch:** `step_3.5` — all work is here.
**Server:** `http://localhost:8000` (via uvicorn), SQLite DB at `services/python-api/marketplace.db`.

---

## Current State (after Stage 3.5)

- FastAPI app with web UI (Jinja2 + Vanilla JS) + JSON API
- SQLite (no Docker required for local dev; PostgreSQL-supported via `DB_URL` env var)
- Auth: register/login with JWT (access + refresh tokens)
- CRUD for users, services, orders
- 23 pytest tests pass, `ruff check .` passes clean
- All files annotated with `# LR #N` (LR = Лабораторная Работа)

---

## Quick Start

```powershell
# From repo root
& "venv/Scripts/Activate.ps1"

# Install deps
pip install -r services/python-api/requirements.txt

# Run server (SQLite auto-creates tables)
cd services/python-api
uvicorn main:app --host 0.0.0.0 --port 8000
```

Open http://localhost:8000/api/ (or http://localhost:8000/ if `API_ROOT_PATH=/api`)

---

## Architecture

### Layered Structure

```
services/python-api/
├── main.py              # App factory: lifespan, middleware, error handlers, router mounting
├── core/
│   ├── config.py        # Pydantic Settings (env vars, defaults to SQLite)
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
│   ├── users.py         # GET /me
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
└── requirements.txt     # fastapi, uvicorn, sqlalchemy, aiosqlite, jinja2, python-jose, passlib, etc.
```

---

## API Reference

### Unified Response Envelope

**Success:** `{"data": ..., "total": N, "limit": N, "offset": N}` (via `core/responses.py`)
**Error:** `{"errors": [{"code": "...", "detail": "..."}]}` (via `core/exceptions.py`)

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/v1/auth/register` | No | Register user. Body: `{email, password, role}` (role: client/provder/admin). Returns tokens + user |
| POST | `/v1/auth/login` | No | Login. Body: `{email, password}`. Returns tokens + user |
| POST | `/v1/auth/refresh` | No | Refresh tokens. Body: `{refresh_token}` |
| GET | `/v1/users/me` | Yes | Current user info |
| GET | `/v1/services/` | No | Published services. Query: `limit`, `offset`, `status` |
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

### Auth

- Header: `Authorization: Bearer <access_token>`
- Tokens expire: access 30 min, refresh 7 days (configurable in `.env`)
- User roles: `client`, `provider`, `admin`

---

## Web UI

### Pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | Catalog (`index.html`) | Fetches `GET /v1/services/` — shows published services in grid, pagination, order button |
| `/login` | Auth (`login.html`) | Register/Login toggle, form validation, calls `handleRegister`/`handleLogin` |
| `/dashboard` | Dashboard (`dashboard.html`) | My services (create/delete), my orders, my sales; auto-create order from catalog params |
| `/escrow/{order_id}` | Escrow Status (`escrow_status.html`) | Visual flow (pending → funded → released), status management buttons |

### JS Client (`static/js/app.js`)

- `apiFetch(path, options)` — base fetch with auth header, JSON parse, error extraction
- `isLoggedIn()`, `getToken()`, `setToken()`, `clearTokens()` — localStorage JWT management
- `fetchServices()`, `fetchMyServices()`, `createService()`, `updateService()`, `deleteService()`
- `fetchOrders()`, `fetchSoldOrders()`, `createOrder()`, `updateOrderStatus()`, `getOrder()`
- `renderServiceCard()`, `renderOrderRow()`, `renderEscrowStatus()`, `renderBadge()`
- `showAlert()`, `clearAlerts()`, `escapeHtml()`
- `renderNavbar()` — dynamic nav (brand, catalog, dashboard, login/logout)
- `API_BASE = '/v1'` constant

### CSS (`static/css/style.css`)

- Custom properties (`--color-primary`, `--color-success`, `--color-danger`, etc.)
- `.container`, `.grid`, `.grid-2`, `.card`, `.card-header`
- `.btn`, `.btn-primary`, `.btn-outline`, `.btn-danger`, `.btn-sm`
- `.badge`, `.badge-draft`, `.badge-published`, `.badge-archived`, `.badge-pending`, `.badge-funded`, `.badge-released`, `.badge-cancelled`, `.badge-disputed`
- `.modal-overlay`, `.modal` — create service modal
- `.escrow-flow`, `.escrow-step`, `.step-dot`, `.escrow-arrow` — escrow status visualization
- `.auth-container`, `.auth-card`, `.auth-switch` — auth form layout
- `.form-group`, `.form-control`, `.form-error` — form styles
- `.table-wrapper` — scrollable table
- `.spinner` — loading animation

---

## Data Model

### User Roles: `client | provider | admin`

### Service Statuses: `draft → published → archived`

- Services default to `draft` on creation
- Only `published` services appear in catalog (`GET /v1/services/` defaults to status=published)
- Provider must update status to `published` via `PUT /v1/services/{id}`

### Order Statuses (Escrow Flow):
```
pending → funded → released
    ↓         ↓
cancelled   disputed
```

---

## Key Fixes Applied This Session (Stage 3.5)

| Problem | Fix | File |
|---------|-----|------|
| 500 error on `/` | TemplateResponse argument order (request first) | `api/v1/ui.py` |
| Static files 404 (hardcoded `/static/...`) | Changed to `url_for('static', path='...')` | `templates/base.html` |
| get_db() didn't persist data | Added `session.commit()` on success, rollback on exception | `core/db.py` |
| Unhandled exceptions silently swallowed | `logging.getLogger().exception()` | `core/exceptions.py` |
| PostgreSQL dependency for local dev | Default DB_URL changed to `sqlite+aiosqlite` | `core/config.py` |
| Tests failed on SQLite | Force SQLite env var, register `now()` on test engine | `tests/conftest.py` |
| "now()" function missing in SQLite | Registered via `event.listen(engine.sync_engine, "connect")` | `core/db.py` |
| Table creation missing | Auto-create tables in `lifespan` (SQLite only) | `main.py` |

---

## Running Tests

```powershell
cd services/python-api
python -m pytest -v
ruff check .
```

---

## Known Issues / Next Steps

- Go escrow service (`services/go-escrow/`) is scaffolded but not implemented
- Blockchain simulator not started
- No production Docker/PostgreSQL setup (currently SQLite-only for local dev)
- UI lacks: password reset, profile editing, admin panel, service search/filter
- Orders don't actually interact with an external escrow system yet
- Push to remote when ready
