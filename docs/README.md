# Marketplace Services with Escrow

Маркетплейс услуг с эскароу-системой на основе микросервисной архитектуры. Три взаимодействующих сервиса обеспечивают полный цикл: каталог услуг, безопасные расчёты через escrow и неизменяемый аудит-трейл в блокчейне.

---

## Архитектура

```
┌──────────────┐     ┌──────────────┐     ┌────────────────┐
│  Python API  │────▶│  Go Escrow   │────▶│ Blockchain Sim │
│  (FastAPI)   │     │   (Gin)      │     │  (FastAPI)     │
│     :8000    │     │    :8081     │     │     :8082      │
└──────┬───────┘     └──────┬───────┘     └────────────────┘
       │                    │
       ▼                    ▼
┌──────────────┐     ┌──────────────┐
│  PostgreSQL  │     │    Redis     │
│     :5432    │     │    :6379     │
└──────────────┘     └──────────────┘
```

### Сервисы

| Сервис | Технологии | Порт | Роль |
|--------|-----------|:----:|------|
| **postgres** | PostgreSQL 15 | 5432 | Основная БД (Python + Go) |
| **redis** | Redis 7 | 6379 | Кеширование escrow-данных |
| **python-api** | FastAPI, SQLAlchemy async, Pydantic V2 | 8000 | Auth, каталог, заказы, wallet, UI |
| **go-escrow** | Gin, state-machine, shopspring/decimal | 8081 | Escrow-счета, dispute, blockchain events |
| **blockchain-sim** | FastAPI, SQLite, SHA-256 | 8082 | Симулятор блокчейна с верификацией |

### Взаимодействие сервисов

- **Python API → Go Escrow**: HTTP (httpx async) — создание escrow, fund, release, dispute
- **Go Escrow → Blockchain Sim**: HTTP (goroutine + retry queue) — запись каждого события в блокчейн
- **Graceful fallback**: при недоступности Go — прямой PATCH в БД из Python

---

## Документация

| Документ | Описание |
|----------|----------|
| [`demo-script.md`](demo-script.md) | Пошаговый сценарий защиты (8 шагов) |
| [`contracts/escrow-api.md`](contracts/escrow-api.md) | REST-контракт Python API ↔ Go Escrow (7 эндпоинтов) |
| [`contracts/blockchain-events.md`](contracts/blockchain-events.md) | REST-контракт Go Escrow ↔ Blockchain Sim |
| [`ai-usage-log.md`](ai-usage-log.md) | Журнал изменений |

### API-документация (Swagger)

| Сервис | URL |
|--------|-----|
| Python API | `http://localhost:8000/docs` |
| Python API (ReDoc) | `http://localhost:8000/redoc` |
| Go Escrow | `http://localhost:8081/swagger/index.html` |

---

## Быстрый старт

```bash
# 1. Запуск всех сервисов
docker compose up -d --build

# 2. Проверка здоровья
docker compose ps
# Все 5 контейнеров должны быть healthy

# 3. Инициализация тестовых данных
cd services/python-api && python scripts/seed_db.py

# 4. Проверка API
curl http://localhost:8000/health
curl http://localhost:8081/health
curl http://localhost:8082/health

# 5. Регистрация и вход
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"client@test.com","password":"pass12345","role":"client"}'

curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"client@test.com","password":"pass12345"}'
```

---

## Команды

```bash
make up             # Сборка и запуск всех сервисов
make down           # Остановка с удалением томов
make test           # Запуск всех тестов (Python + Go)
make test-python    # Только Python тесты
make test-go        # Только Go тесты
make lint           # pre-commit (black, ruff, gofmt, bandit)
make sast           # bandit + gosec
make build          # docker compose build
```

---

## Тестирование

| Сервис | Тестов | Coverage |
|--------|:------:|:--------:|
| **Python API** | 80 | 80% |
| **Go Escrow** | 161 | 80.5% |
| **Blockchain Sim** | 26 | 99% |

### Go coverage по пакетам

| Пакет | Coverage |
|-------|:--------:|
| `domain` (state-machine) | 100% |
| `config` | 100% |
| `repository` | 97.3% |
| `clients` (blockchain) | 91.7% |
| `service` (business logic) | 89.4% |
| `api` (handlers) | 88.5% |
| `db` | 76.7% |

---

## Ключевые возможности

### Python API (FastAPI)
- JWT аутентификация (access + refresh tokens, 3 роли: client, provider, admin)
- REST API: каталог услуг, заказы, кошелёк
- Pydantic V2 валидация с `condecimal(max_digits=19, decimal_places=4)`
- CORS, X-Request-ID, structlog
- Jinja2 UI: каталог, dashboard, wallet, админ-панель

### Go Escrow (Gin)
- State-machine с 8 статусами: `CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED`
- Валидация всех переходов через `IsValidTransition` (O(1) map lookup)
- X-Idempotency-Key защита от дублирования
- Token-bucket rate-limiter на fund
- Graceful shutdown

### Blockchain Simulator (FastAPI)
- SHA-256 хеширование блоков
- Связная цепочка: каждый блок хранит `prev_hash`
- SQLite-персистентность с верификацией при загрузке
- Обнаружение подмены данных (chain integrity check)

### Финансовая точность
- Все суммы — `DECIMAL`/`NUMERIC(19,4)`, никаких `float`
- Python: `condecimal` + `Numeric(19,4)`
- Go: `shopspring/decimal` + `NUMERIC(19,4)`
- PostgreSQL: `CHECK (balance >= 0)`, `CHECK (amount > 0)`

### Безопасность
- SAST: bandit (0 High/Medium), gosec (0 issues)
- Non-root пользователи во всех Docker-образах
- Prepared statements (SQLAlchemy ORM + Go `$N`)
- JWT с bcrypt

---

## Структура проекта

```
.
├── .github/workflows/ci.yml    # GitHub Actions: lint → test → sast
├── docker-compose.yml           # 5 сервисов
├── Makefile                     # up, down, test, lint, sast, build
├── .pre-commit-config.yaml      # black, ruff, gofmt, bandit
├── .env.example                 # Шаблон переменных окружения
├── docs/
│   ├── README.md
│   ├── demo-script.md
│   ├── contracts/
│   │   ├── escrow-api.md
│   │   └── blockchain-events.md
│   └── ai-usage-log.md
└── services/
    ├── python-api/              # FastAPI + SQLAlchemy async
    │   ├── api/v1/              # auth, users, services, orders, wallet, escrow, chain
    │   ├── app/services/        # escrow_client, escrow_cache
    │   ├── core/                # config, db, security, exceptions, deps, responses
    │   ├── models/              # users, services, orders, wallet
    │   ├── schemas/             # pydantic-схемы с condecimal
    │   ├── repositories/        # CRUD-слой
    │   ├── templates/           # Jinja2 UI (15 шаблонов)
    │   ├── static/              # CSS (8) + JS (13) — vanilla
    │   ├── alembic/             # 7 миграций
    │   └── tests/               # 80 тестов
    ├── go-escrow/               # Gin + state-machine
    │   ├── internal/
    │   │   ├── api/             # handlers, middleware, router, idempotency
    │   │   ├── clients/         # blockchain_client (retry queue)
    │   │   ├── config/
    │   │   ├── db/
    │   │   ├── domain/          # state-machine (8 статусов)
    │   │   ├── repository/      # SQL с prepared statements
    │   │   ├── service/         # бизнес-логика + blockchain events
    │   │   └── testutil/
    │   └── cmd/healthcheck/
    └── blockchain-sim/          # FastAPI + SHA-256 + SQLite
        ├── blockchain.py        # Block/Blockchain цепочка
        ├── database.py          # SQLite persistence
        └── tests/               # 26 тестов, 99% coverage
```

---

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`):

| Шаг | Инструменты |
|-----|-------------|
| **Lint** | ruff, go vet |
| **Test** | pytest (81), go test (161) |
| **SAST** | bandit (0 High/0 Medium), gosec (0 issues) |

Git Flow: `master` + `step_*` ветки, Conventional Commits.

---

## License

Учебный проект. Курсовая работа 6 семестра — Методы и технологии программирования.
