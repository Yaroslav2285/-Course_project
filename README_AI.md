# README for AI

## Project: Service Marketplace (Маркетплейс услуг)

Monorepo with Python (FastAPI) backend + Go escrow service + blockchain simulator.

---

## Current State (2026-05-26, branch `step_3.5`)

- FastAPI app works locally with SQLite (no Docker required)
- Web UI: catalog, login, dashboard, escrow status pages
- Auth: register/login with JWT tokens
- CRUD: services, orders, users
- 23 pytest tests pass (91% coverage)
- Static files served via FastAPI (no npm/build step)

## Quick Start

```powershell
# Activate venv & install deps
& "venv/Scripts/Activate.ps1"
pip install -r services/python-api/requirements.txt

# Run server
cd services/python-api
uvicorn main:app --host 0.0.0.0 --port 8000
```

Open http://localhost:8000/api/ (or http://localhost:8000/ without `API_ROOT_PATH`)

## Key Files

| Path | Purpose |
|---|---|
| `services/python-api/main.py` | App factory, lifespan, middleware |
| `services/python-api/core/db.py` | SQLite engine, `get_db()` with commit |
| `services/python-api/core/security.py` | JWT password hashing |
| `services/python-api/api/v1/` | API routers (auth, services, orders, users) |
| `services/python-api/api/v1/ui.py` | Web UI routes + `mount_static()` |
| `services/python-api/templates/` | Jinja2 templates (base, index, login, dashboard, escrow_status) |
| `services/python-api/static/` | CSS (`style.css`) + JS (`app.js`) |
| `services/python-api/tests/` | Async pytest suite |
| `services/go-escrow/` | Go escrow service (TODO) |
| `docs/ai-usage-log.md` | AI session history |

## .env (`services/python-api/.env`)

```
DB_URL=sqlite+aiosqlite:///./marketplace.db
JWT_SECRET=your-super-secret-jwt-key-change-in-production-min-32-chars
API_ROOT_PATH=/api
```

## API Endpoints (prefix: `POST /v1/auth/register`, `POST /v1/auth/login`, `GET /v1/services/`, `POST /v1/services/`, `GET/PUT/DELETE /v1/services/{id}`, `POST /v1/orders/`, `GET /v1/orders/`, `GET /v1/orders/{id}`, `GET /v1/escrow/{id}/status`, `GET /v1/users/me`)

- All responses: `{"data": ..., "total": ..., "limit": ..., "offset": ...}`
- Auth header: `Authorization: Bearer <token>`
- Error format: `{"errors": [{"detail": "...", "code": "..."}]}`

## Running Tests

```powershell
cd services/python-api
python -m pytest -v
ruff check .
```

## Conventions

- All Python files annotated with `# LR #N` comments (LR = Лабораторная Работа)
- SQLAlchemy async sessions, Pydantic V2 schemas
- Prices use `condecimal` (19,4)
- Services default to `draft` status; only `published` shows in catalog
- commit `get_db()` now commits on success, rolls back on exception
- SQLite table creation on startup (no Alembic for local dev)

## Known Issues / Next Steps

- Go escrow service (`services/go-escrow/`) is scaffolded but not implemented
- Blockchain simulator not started
- No production PostgreSQL setup (currently SQLite-only)
- Push to remote when ready
