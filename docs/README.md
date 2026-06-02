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
- **Python API → Blockchain Sim**: HTTP (httpx async) — аудит блокчейна при загрузке заказов
- **Graceful fallback**: при недоступности Go — прямой PATCH в БД из Python; при недоступности Blockchain Sim — in-memory mock

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

# 3. Запуск без Docker (только Python API для разработки)
cd services/python-api
cp .env.example .env      # настроить DB_URL, REDIS_URL
python main.py            # uvicorn с reload_excludes
# Требуются: PostgreSQL + Redis (docker compose up -d postgres redis)

# 4. Инициализация тестовых данных
cd services/python-api && python scripts/seed_db.py

# 5. Проверка API
curl http://localhost:8000/health
curl http://localhost:8081/health
curl http://localhost:8082/health

# 6. Регистрация и вход
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

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `DB_URL` | `postgresql+asyncpg://app_user:ChangeMe123!@localhost:5432/app_db` | URL подключения к PostgreSQL |
| `REDIS_URL` | `redis://redis:6379/0` | URL подключения к Redis |
| `JWT_SECRET` | — | Секрет для подписи JWT |
| `PYTHON_API_PORT` | `8000` | Порт Python API |
| `GO_ESCROW_BASE_URL` | `http://go-escrow:8081` | URL для Go Escrow сервиса |
| `BLOCKCHAIN_SIM_BASE_URL` | `http://blockchain-sim:8082` | URL для Blockchain Simulator |
| `BLOCKCHAIN_DB_PATH` | `blockchain.db` | Путь к SQLite БД блокчейна |
| `BLOCKCHAIN_SIM_PORT` | `8082` | Порт Blockchain Simulator |

---

## Ключевые возможности

### Python API (FastAPI)

- **JWT аутентификация** (access + refresh tokens, 3 роли: client, provider, admin)
- **REST API**: каталог услуг, заказы (CRUD с пагинацией), кошелёк (пополнение, оплата, перевод, баланс, транзакции)
- **Pydantic V2 валидация** с `condecimal(max_digits=19, decimal_places=4)` — все суммы точные
- **Timeout middleware** (25s → HTTP 504) — защита от зависших запросов
- **Connection pool warming** — 3× `SELECT 1` при старте для прогрева пула asyncpg
- **CORS, X-Request-ID**, structlog
- **Jinja2 UI**: каталог (`/catalog`), dashboard (`/dashboard`), wallet (`/wallet`), админ-панель (`/admin/disputes`)
- **Alembic миграции** (7 миграций, включая wallet, orders, services, users)

### Go Escrow (Gin)

- **State-machine с 8 статусами**: `CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED`, с поддержкой `CANCELLED`, `DISPUTED`, `RESOLVED`
- **Валидация всех переходов** через `IsValidTransition` (O(1) map lookup)
- **X-Idempotency-Key** защита от дублирования операций
- **Token-bucket rate-limiter** на fund (предотвращает флуд)
- **Retry queue** для blockchain-событий (до 3 попыток с exponential backoff)
- **Graceful shutdown** (ожидание завершения активных операций)

### Blockchain Simulator (FastAPI)

- **SHA-256 хеширование блоков** — каждый блок содержит `prev_hash` для связности
- **Связная цепочка**: обнаружение подмены данных (chain integrity check)
- **SQLite-персистентность** с верификацией при загрузке
- **Автовосстановление**: при отсутствии данных создаётся genesis-блок

### Финансовая точность

- Все суммы — `DECIMAL`/`NUMERIC(19,4)`, никаких `float`
- Python: `condecimal(max_digits=19, decimal_places=4)` + `Numeric(19,4)`
- Go: `shopspring/decimal` + `NUMERIC(19,4)`
- PostgreSQL: `CHECK (balance >= 0)`, `CHECK (amount > 0)`

### Веб-интерфейс

- **15 HTML-шаблонов** (Jinja2): каталог, dashboard, wallet, escrow-панель, заказы, админка, аутентификация
- **13 JS-файлов** (vanilla): `api.js` (Promise.race + retry), `auth.js`, `dashboard.js`, `catalog.js`, `order-modal.js`, `wallet.js`, `escrow-panel.js`, `admin.js`
- **8 CSS-файлов**: адаптивный дизайн, CSS-переменные, анимации

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

### Запуск тестов

```bash
# Все тесты
make test

# Python
cd services/python-api && pytest -v

# Go
cd services/go-escrow && go test ./... -v -cover

# Blockchain Sim
cd services/blockchain-sim && pytest -v
```

---

## Безопасность

### SAST (Static Application Security Testing)

- **bandit**: 0 High, 0 Medium уязвимостей
- **gosec**: 0 issues во всех пакетах Go

### Практики

- **Non-root пользователи** во всех Docker-образах (`appuser`)
- **Prepared statements**: SQLAlchemy ORM (Python) + `$N` параметры (Go)
- **JWT с bcrypt**: пароли не хранятся в открытом виде
- **Параметризованные SQL-запросы** (SQLite в blockchain-sim)
- **Лимитирование**: rate-limiter на fund (Go), timeout middleware (Python)
- **CORS**: настроен на `http://localhost:8000`
- **X-Request-ID**: сквозной идентификатор запроса через все сервисы

### CI/CD Security

- GitHub Actions: `lint → test → sast` на каждый push
- pre-commit хуки: `black`, `ruff`, `gofmt`, `bandit`

---

## Персистентность данных

### Тома Docker

| Том | Сервис | Назначение |
|-----|--------|------------|
| `postgres_data` (`marketplace_postgres_data`) | postgres | Основная БД (PostgreSQL) |
| `blockchain_sim_data` (`marketplace_blockchain_sim_data`) | blockchain-sim | Блокчейн-цепочка (SQLite) |

### Восстановление после потери данных блокчейна

При утере БД блокчейна (например, `docker compose down -v`) заказы автоматически восстанавливаются:

1. `_check_blockchain_audit` в `orders.py` запрашивает `/v1/chain/audit/{id}`
2. Если blockchain-sim возвращает **404**, а статус заказа — `funded`, `in_progress`, `completed`, `released`, `disputed`, `resolved` и т.д. (гарантированно был в escrow)
3. API отправляет `POST /v1/chain/submit` с `action: "restored"` и данными заказа
4. Создаётся новый блок, заказ получает зелёную галочку

> `cancelled` не восстанавливается — он мог быть отменён до фандинга, без блокчейн-записи.

### In-memory кеш аудита

`_blockchain_cache` (dict с TTL 5 минут) предотвращает повторные запросы к blockchain-sim.

---

## Ключевые архитектурные решения

| Решение | Обоснование |
|---------|------------|
| **PostgreSQL вместо SQLite** | SQLite `StaticPool` не справлялся с конкурентными запросами; PostgreSQL + asyncpg даёт пул соединений |
| **`asyncio.gather` для заказов** | Параллельная обработка заказов (с `selectinload`) вместо последовательного `for` |
| **`Promise.race` вместо `AbortController`** | Chrome прерывал fetch-сигналы при навигации, `AbortSignal` выбрасывал "signal is aborted without reason" |
| **Timeout middleware 25s → 504** | Защита от зависших запросов без таймаута |
| **Pool warmup (3× SELECT 1)** | Прогрев пула asyncpg на старте для быстрого первого запроса |
| **Blockchain volume** | SQLite внутри контейнера теряется при рестарте; именованный volume сохраняет блокчейн |
| **Restore on 404** | Заказы, созданные до volume-фикса, не теряют галочку |
| **`For` → `asyncio.gather` revert** | После добавления `selectinload` гонки ORM больше нет, gather безопасен |

---

## Известные проблемы

### First-load timeout после логина

На Windows при первом входе после регистрации/логина все API-запросы (`/v1/orders/`, `/v1/wallet/balance`, `/v1/wallet/transactions`) уходят в 15-секундный таймаут (`Error TIMEOUT` от `Promise.race`).

**Причина**: предположительно гонка TCP-соединений при редиректе (login → dashboard), характерная для Windows-сокетов.

**Workaround**: один F5 (обновление страницы) решает проблему для всех последующих загрузок. `_blockchain_cache` заполняется, TCP-соединения стабилизируются.

### Blockchain Simulator недоступен

Если `blockchain-sim` не запущен, каждый запрос аудита ждёт `asyncio.wait_for` 3 секунды, после чего кеширует `False`. Это замедляет загрузку дашборда.

**Решение**: запустить `docker compose up -d blockchain-sim` или, при локальной разработке, убедиться что `BLOCKCHAIN_SIM_BASE_URL` указывает на работающий экземпляр.

---

## Структура проекта

```
.
├── .github/workflows/ci.yml    # GitHub Actions: lint → test → sast
├── docker-compose.yml           # 5 сервисов + 2 named volumes
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
    │   ├── api/v1/              # auth, users, services, orders, wallet, escrow, chain, admin, ui
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
        ├── database.py          # SQLite persistence + makedirs
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
pre-commit хуки запускаются локально перед каждым коммитом.

---

## License

Учебный проект. Курсовая работа 6 семестра — Методы и технологии программирования.
