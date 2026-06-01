# Marketplace with Escrow System — Полный контекст проекта

## Монорепозиторий

```
Project/
├── docker-compose.yml       # 5 сервисов: postgres, redis, python-api, go-escrow, blockchain-sim
├── Makefile                 # test, lint, sast, build цели
├── .env.example             # шаблон переменных окружения
├── .gitignore               # README_AI.md в списке игнорируемых
├── scripts/
│   └── seed_db.py           # сидирование: регистрация → услуга → публикация → заказ
├── docs/
│   ├── contracts/
│   │   ├── escrow-api.md          # REST контракт Python ↔ Go
│   │   └── blockchain-events.md   # REST контракт Go ↔ Blockchain
│   └── ai-usage-log.md            # полный журнал изменений с AI
├── services/
│   ├── python-api/          # FastAPI · порт 8000
│   ├── go-escrow/           # Gin · порт 8081
│   └── blockchain-sim/      # FastAPI · порт 8082
└── infra/                   # резерв для Kubernetes-манифестов
```

## Архитектура

```
Client (curl/browser)
    │
    ▼
┌─────────────────┐     HTTP/REST      ┌──────────────────┐
│   Python API    │ ←─────────────────→ │    Go Escrow     │
│   :8000         │   X-Request-ID      │    :8081          │
│   Orders/Auth   │   X-Idempotency-Key │   State-machine   │
│   FastAPI       │                     │   Rate-limiter    │
│   SQLAlchemy    │                     │   Gin             │
└────────┬────────┘                     └────────┬─────────┘
         │                                       │
         ▼                                       ▼
┌──────────────────┐                   ┌──────────────────┐
│   PostgreSQL     │                   │   PostgreSQL     │
│   :5432          │                   │   (same DB)      │
│   users          │                   │   escrow_accounts │
│   services       │                   │   transactions    │
│   orders         │                   │   disputes        │
└──────────────────┘                   └────────┬─────────┘
                                                │ (async goroutine)
                                                ▼
                                       ┌──────────────────┐
                                       │ Blockchain Sim   │
                                       │   :8082          │
                                       │   SHA-256 chain  │
                                       │   SQLite         │
                                       └──────────────────┘

Redis :6377 — кеширование (зарезервировано)
```

## Что создано на каждом этапе

### Этап 1 — Инфраструктура
- `docker-compose.yml` — 5 сервисов с networks, volumes, healthcheck
- `Makefile` — `up`, `down`, `test`, `lint`, `sast`, `build`
- `.env.example` — шаблон с дефолтными значениями
- `.gitignore`, `.editorconfig`, `.gitattributes`, `.pre-commit-config.yaml`
- `.github/workflows/ci.yml` — GitHub Actions: lint (ruff, go vet), test (pytest, go test), SAST (bandit, gosec) на push/PR
- **Проверка Phase 1: 7/7 ✅**
- **Проверка Phase 2: models/migrations — 6/6 ✅** (найден и исправлен VARCHAR(11)→VARCHAR(20) для `transactions.type`)
- **Проверка Phase 3: repositories/API — 28/28 эндпоинтов ✅** (исправлен regex статусов, DB-level фильтрация)
- **Проверка Phase 4: blockchain-sim — 4 эндпоинта, 26 тестов, 99% coverage ✅**
- **Проверка Phase 5: testing — 81 тест, 80% Python coverage ✅**

### Этап 2 — Модели данных и миграции

**Python (SQLAlchemy async + Alembic):**
- `models/base.py` — DeclarativeBase
- `models/users.py` — User(id, email, hashed_password, role, created_at, updated_at)
- `models/services.py` — Service(id, provider_id FK→users, title, description, **price NUMERIC(19,4)**, status, timestamps)
- `models/orders.py` — Order(id, service_id FK→services, buyer_id/seller_id FK→users, **amount NUMERIC(19,4)**, status, notes, timestamps)
- `alembic/env.py` — async, читает DB_URL из окружения
- Миграция `fff36bd858b0_init.py` — создаёт все 3 таблицы, `DROP TABLE IF EXISTS goose_db_version`

**Go (database/sql + embed):**
- `internal/db/migrations/001_initial.sql` — `escrow_accounts` (CHECK balance>=0), `transactions` (CHECK amount>0), `disputes`
- Встраивается через `//go:embed` и выполняется при старте (`runMigrations`)

### Этап 3 — Python API Core

**Core:**
- `core/config.py` — Pydantic Settings, чтение из env
- `core/db.py` — async engine + session factory, SQLite для тестов / PostgreSQL для прода
- `core/security.py` — `hash_password`/`verify_password` (bcrypt 4.2.1), JWT access+refresh (python-jose)
- `core/exceptions.py` — кастомные исключения + обработчики 400/401/403/404/409/500
- `core/deps.py` — `get_db`, `get_current_user` dependency
- `core/responses.py` — единый формат `{"data", "meta", "errors"}`

**API эндпоинты (все под `/v1`):**
| Метод | Путь | Описание |
|---|---|---|
| POST | `/v1/auth/register` | Регистрация, возвращает JWT |
| POST | `/v1/auth/login` | Логин, access + refresh |
| POST | `/v1/auth/refresh` | Обновление access токена |
| GET | `/v1/users/me` | Профиль текущего пользователя |
| PUT | `/v1/users/me` | Обновление профиля |
| GET/POST | `/v1/services/` | Список (только published) / создание |
| GET/PUT/DELETE | `/v1/services/{id}` | CRUD услуги (владелец — только свои) |
| GET/POST | `/v1/orders/` | Список заказов / создание |
| PATCH | `/v1/orders/{id}/status` | Обновление статуса (pending→funded→in_progress→completed→released|cancelled|disputed) |
| GET | `/v1/orders/sold` | Проданные заказы |
| GET | `/v1/orders/{id}` | Детали заказа |
| GET | `/v1/wallet/balance` | Баланс кошелька (ленивое создание) |
| POST | `/v1/wallet/topup` | Пополнение кошелька |
| POST | `/v1/wallet/pay` | Оплата заказа из кошелька (→funded) |
| GET | `/v1/wallet/transactions` | История транзакций |
| GET | `/admin/disputes` | Список споров (admin) |
| POST | `/admin/disputes/{id}/release` | Разрешить спор → продавцу (admin) |
| POST | `/admin/disputes/{id}/refund` | Разрешить спор → покупателю (admin) |

**Middleware:**
- X-Request-ID (проброс + эхо)
- CORS
- structlog JSON-логирование
- Error handlers (ValidationError, HTTPException, unhandled)

**UI (jinja2):**
- `api/v1/ui.py` — conditional import (если нет jinja2 — UI не загружается)
- 5 шаблонов: index, login, dashboard, escrow_status

**Тесты:** 32/32, 91% покрытие (api, core, models, repositories)

**Frontend (Этапы 7.1–7.2):** смотри ниже

### Этап 4 — Go Escrow Microservice

**Структура:**
```
internal/
├── config/config.go        # PgDsn, Port, BlockchainSimURL из env
├── db/db.go                # Connect(), runMigrations(), WithTx(), pool 25/5 conn
├── domain/escrow.go        # EscrowAccount, EscrowStatus enum, state-machine (7 статусов)
├── repository/escrow_repo.go # prepared statements (Create, GetByID, UpdateBalanceAndStatus)
├── service/escrow_service.go # бизнес-логика + вызов blockchain клиента
├── api/
│   ├── handler.go          # Create, Fund, Release, Dispute, Advance, GetByID
│   ├── router.go           # маршруты + группы + middleware
│   ├── middleware.go        # RateLimiter (token-bucket per IP), CORS, X-Request-ID, Logger, Recovery
│   └── idempotency.go      # X-Idempotency-Key store (in-memory + TTL + goroutine cleanup)
└── clients/
    └── blockchain_client.go # HTTP клиент с retry (3 attempts + jitter), асинхронная очередь
cmd/healthcheck/main.go     # Go healthcheck бинарник для distroless Docker
```

**State-machine (EscrowStatus):**
```
CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED
             ↓           ↓              ↘
             ↓           ↓            DISPUTED
             ↓           ↓                ↓
             ↓        CANCELLED        RESOLVED
             ↓
          CANCELLED
```
Валидные переходы: карта `ValidTransitions`, невалидный → 409 INVALID_TRANSITION. CANCELLED — терминальный статус (выхода нет).

**REST API (все под `/v1`):**
| Метод | Путь | Описание |
|---|---|---|
| POST | `/escrow/` | Создать escrow |
| GET | `/escrow/{id}` | Получить по ID |
| POST | `/escrow/{id}/fund` | Пополнить (rate-limited) |
| POST | `/escrow/{id}/release` | Высвободить |
| POST | `/escrow/{id}/cancel` | Отменить (FUNDED/IN_PROGRESS → CANCELLED) |
| POST | `/escrow/{id}/dispute` | Открыть спор (требует reason) |
| POST | `/escrow/{id}/advance` | Продвинуть статус (FUNDED→IN_PROGRESS→COMPLETED) |
| GET | `/health` | Healthcheck |

**Тесты:** ~116 test functions across 7 packages: domain (50+), service (13), handler (16), middleware (12), idempotency (9), router (6), clients (14, incl. 9 retry queue), repository (21, on step_4), config (4), db (4), postgres integration (3), top-level integration (2). **Покрытие по пакетам:** domain ~95%, service ~75%, handler ~70%, middleware/idempotency ~85%, router ~90%, clients ~55%, repository ~80% (step_4), config ~100%, db ~100%. **Общая оценка: ~70-80%**

### Этап 5 — Blockchain Simulator

**Структура:**
```
services/blockchain-sim/
├── config.py        # порт из env
├── schemas.py       # Pydantic V2 модели (SubmitRequest, BlockResponse и т.д.)
├── blockchain.py    # Block (SHA-256, nonce, prev_hash), Blockchain (add, is_valid, audit)
├── database.py      # SQLite с prepared statements
├── main.py          # FastAPI с structlog
└── tests/           # 26 тестов, 99% покрытие
```

**API:**
| Метод | Путь | Описание |
|---|---|---|
| POST | `/v1/chain/submit` | Добавить событие в блокчейн |
| GET | `/v1/chain/verify?block_index=` | Верифицировать цепочку (SHA-256 prev_hash) |
| GET | `/v1/chain/audit/{order_id}` | Получить аудит-трейл по заказу |
| GET | `/health` | Healthcheck |

**Тесты:** 26/26, 99% покрытие

### Этап 6 — Интеграция и межсервисные клиенты

**Документация контрактов:**
- `docs/contracts/escrow-api.md` — полный REST контракт Python ↔ Go
- `docs/contracts/blockchain-events.md` — полный REST контракт Go ↔ Blockchain

**Python → Go (escrow_client.py):**
- `app/services/escrow_client.py` — async httpx клиент
- Retry: 3 попытки, exponential backoff
- X-Request-ID forwarding
- X-Idempotency-Key поддержка
- Таймаут: 5s
- Методы: create_escrow, fund_escrow, release_escrow, cancel_escrow, dispute_escrow, advance_escrow, complete_escrow
- Error mapping: 404→NotFound, 409→Conflict, остальное→ServiceError

**Go → Blockchain (blockchain_client.go):**
- HTTP клиент с context timeout
- zap логирование
- Retry: 3 попытки + jitter
- Асинхронная очередь (goroutine-based, неблокирующая)
- Вызов `emitBlockchainEvent` после каждого перехода статуса (CREATED, FUNDED, RELEASED, DISPUTED)

**Интеграционные тесты:**
- Python: 8 тестов (respx моки) — create, fund, release, dispute, 404, 409, retry, idempotency, unavailable
- Go: 2 E2E теста — полный цикл + dispute цикл (с mock blockchain сервером)

### Этап 7.1 — Pure HTML/CSS/JS Frontend Scaffold + Auth

**Стек:** Чистый HTML5, CSS3, Vanilla JS. Никаких фреймворков, CDN или npm.
FastAPI отдаёт статику через `StaticFiles` + `Jinja2Templates`.

**Структура:**
```
services/python-api/
├── static/
│   ├── css/
│   │   └── main.css          # CSS-переменные, navbar, buttons, forms, cards, toast, badges, modal, responsive
│   └── js/
│       ├── api.js             # apiFetch wrapper с JWT, auto-401/403, все endpoint wrappers
│       ├── utils.js           # showToast, showAlert, debounce, escapeHtml, renderBadge
│       ├── auth.js            # saveAuthData, checkAuth, handleLogin/Register/Logout, updateNavbar
│       ├── app.js             # DOMContentLoaded entry point, page-specific init (login/register)
│       └── dashboard.js       # Dashboard logic: client orders, executor services/orders CRUD
├── templates/
│   ├── base.html             # Jinja2 base: navbar, toast-container, script slots
│   ├── index.html            # Landing page
│   ├── login.html            # Login form с role-based redirect
│   ├── register.html         # Registration form с role selection
│   ├── dashboard.html        # Dashboard общий
│   ├── dashboard_client.html # Dashboard для client
│   ├── dashboard_executor.html # Dashboard для executor
│   ├── catalog.html          # Каталог (Phase 7.2)
│   ├── orders.html           # Список заказов
│   ├── order_detail.html     # Детали заказа
│   └── audit.html            # Аудит-трейл блокчейна
└── api/v1/ui.py              # 12 UI-маршрутов + mount_static()
```

**Ключевые решения:**
- Auth: JWT access+refresh токены в `localStorage`
- Ролевой редирект: client → `/dashboard/client`, executor → `/dashboard/executor` (никогда на `/`)
- Статика: `/api/static/...` (из-за `root_path="/api"` в конфиге)
- Комментарии: `# LR #6`, `# LR #7`, `# LR #8`, `# LR #10`, `# LR #12`, `# LR #15`

### Этап 7.2 — Каталог услуг с фильтрацией и пагинацией

**Файлы:**
- `templates/catalog.html` — сайдбар (поиск, диапазон цен, сортировка) + сетка карточек + пагинация + skeleton/empty/error контейнеры
- `static/css/catalog.css` — sticky-сайдбар, CSS Grid `repeat(auto-fill, minmax(280px, 1fr))`, .service-card hover, skeleton shimmer-анимация, responsive (мобильный сайдбар toggle)
- `static/js/catalog.js` — полная клиентская логика: fetch из `/v1/services/`, фильтрация (search, price, sort), пагинация, роль-aware кнопка Order

**Функциональность:**
- Загрузка: скелетоны (shimmer-анимация) → реальные данные / пусто / ошибка
- Фильтры: поиск по названию/описанию, диапазон цен, сортировка (newest, oldest, price_asc, price_desc, title_asc)
- Пагинация: Prev/Next + номера страниц, smooth scroll
- Кнопка Order: скрыта для executor (disabled с tooltip), ведёт на `/register` для неавторизованных
- URL sync: `history.pushState` при применении фильтров
- Debounce на поиск (300ms), валидация цены (min ≤ max)
- Обработка ошибок: toast + retry button, auto-redirect на `/login` при 401

**Технические детали:**
- `apiFetchServices({ limit: 100 })` — запрос всех published услуг (бэкенд не поддерживает поиск/сортировку)
- Все фильтры и пагинация — клиентские (JavaScript)
- Price: `Intl.NumberFormat` / `parseFloat().toFixed(2)`
- `checkAuth()` из `auth.js` — карточки адаптируются под роль

### Этап 7.3 — Панель управления (Dashboard) и управление заказами

**Файлы (рефакторинг):**
- `api/v1/escrow_proxy.py` — (новый) прокси-роутер для `/v1/escrow/:order_id/:action`, пытается Go escrow → fallback на прямой PATCH статуса заказа
- `app/services/escrow_client.py` — добавлены `advance_escrow()` и `complete_escrow()`
- `static/css/dashboard.css` — (новый, 435 строк) стили для 7 escrow-статусов, skeleton shimmer-анимации, escrow timeline, action-кнопок
- `static/js/dashboard.js` — (переписан, 519 строк) полная JS-логика с escrow state-machine (fund→advance→complete→release), debounce на клики, optimistic обновление UI через `updateOrderStatusUI()`
- `templates/dashboard_client.html` — переписан с состояниями loading/empty/error/skeleton для каждого escrow-статуса
- `templates/dashboard_executor.html` — переписан с табами (My Services / Incoming Orders), модальным CRUD услуг, escrow-кнопками
- `templates/base.html` — добавлен `dashboard.css`, кэш-бастинг (`?v=2`) на все CSS/JS

**Дополнительные изменения:**
- **Footer**: добавлен `<footer>` блок с копирайтом во все страницы через `base.html`
- **Navbar**: статический HTML (Login/Register) с JS-переключением на аутентифицированное состояние (Catalog+Dashboard+user+Logout), ссылки выровнены вправо (`margin-left: auto`)
- **Logout**: редирект на `/` (главную) вместо `/login`
- **Landing page**: убрана кнопка "Browse Services"
- **Python 3.14**: исправлена несовместимость pydantic-core — `pydantic>=2.11.2` (unpinned) в `requirements.txt`

**Функциональность панели клиента:**
- Загрузка: skeleton → таблица заказов / пусто / ошибка (всё с защитным try-catch)
- Статусы: pending (желтый), funded (синий), in_progress (оранжевый), completed (зеленый), released (зеленый), disputed (розовый), cancelled (красный)
- Действия по статусу:
  - **pending**: Pay (→funded), Cancel (→cancelled)
  - **funded**: ожидание исполнителя
  - **in_progress**: Dispute (→disputed)
  - **completed**: Confirm Release (→released), Dispute
  - **released**: "✓ Completed"
  - **disputed**: ссылка на `/audit/:id`
  - **cancelled**: "Cancelled"
- Service title подгружается через `GET /v1/services/:id` с кешированием (N+1, на 50 заказов)
- Escrow-прокси: сначала Go escrow (с X-Idempotency-Key), при недоступности — PATCH статуса через fallback + toast warning

**Функциональность панели исполнителя:**
- Табы: My Services / Incoming Orders (счётчики)
- My Services таблица: Title, Price, Status badge, Edit/Delete
- + New Service: модальное окно (title, description, price, status draft/published)
- Edit: предзаполненная форма через index-based массив (без XSS в inline onclick)
- Delete: подтверждение → `DELETE /v1/services/:id`
- Incoming Orders: статусы с действиями:
  - **pending**: ожидание оплаты
  - **funded**: Accept & Start (→in_progress), Cancel
  - **in_progress**: Complete Work (→completed), Cancel
  - **completed**: ожидание подтверждения клиента
  - **released**: "✓ Payment received"
  - **disputed**: ссылка на `/audit/:id`

**Технические детали:**
- Escrow proxy: in-memory `order_id→escrow_id` cache (теряется при рестарте, достаточно для dev)
- Fallback: при недоступности Go escrow (`e.status === 404 || e.status === 0`) — прямой PATCH `/v1/orders/:id/status`
- Debounce: `handleEscrowAction()` блокирует повторные клики через `_actionInProgress[orderId]`
- XSS-безопасность: данные в inline onclick передаются через числовой индекс в `serviceData[]`, а не строкой
- `editServiceByIndex(idx)` — читает данные из массива, избегая экранирования
- `fetchServiceTitle(serviceId)` — кеширует результаты в `serviceCache{}`
- Модальное окно: `.modal-overlay` с click-outside закрытием, чистка формы после Create/Edit
- `apiFetchOrders(50, 0)` — запрос до 50 заказов (без пагинации на UI)
- Cache-busting: `?v=2` на всех CSS/JS для сброса браузерного кэша при деплое
- Все функции обёрнуты в try-catch с `console.error` для диагностики

### Этап 7.6 — Wallet module

**Файлы:**
- `models/wallet.py` — Wallet (user_id, balance) + Transaction (wallet_id, type, amount, status, reference_id). Enum-ы: TransactionType (topup, payment, refund, withdrawal), TransactionStatus (pending, completed, failed)
- `schemas/wallet.py` — Pydantic схемы: TopUpRequest, WalletRead, TransactionRead, TransactionList
- `repositories/wallets.py` — WalletRepository (get_or_create с IntegrityError fix от race condition), TransactionRepository
- `api/v1/wallet.py` — 3 эндпоинта: GET /balance, POST /topup, GET /transactions
- `static/js/wallet.js` — фронтенд: initWallet() загружает баланс, topup-модалка с валидацией, история транзакций с рендером таблицы, polling (5s)
- `static/js/auth.js` — ссылка Wallet в navbar (между Catalog и Dashboard)
- `static/css/wallet.css` — стили: карточка баланса, skeleton, topup-модалка, таблица
- `templates/wallet-card.html` — фрагмент с балансом + topup-модалка (история операций только на /wallet)
- `templates/wallet.html` — новая страница /wallet, наследует base.html, включает wallet-card.html + историю
- `alembic/versions/8b57a29864d5_add_wallet.py` — миграция: таблицы wallets + transactions
- `tests/test_wallet.py` — 10 тестов: create wallet, topup, balance, transactions, race condition
- `requirements.txt` — fastapi 0.136.3, pytest 9.0.3, pytest-asyncio 1.4.0 (чинит deprecation warnings)

**API эндпоинты:**
| Метод | Путь | Описание |
|---|---|---|
| GET | `/v1/wallet/balance` | Баланс кошелька (создаётся при первом запросе) |
| POST | `/v1/wallet/topup` | Пополнение баланса (amount > 0) |
| GET | `/v1/wallet/transactions` | История транзакций (пагинация limit/offset) |

**Ключевые решения:**
- Кошелёк создаётся лениво (lazy creation) при первом запросе баланса через `get_or_create`
- Race condition в `get_or_create`: исправлено через `try/except IntegrityError` с `rollback()` + повторный `SELECT`
- Отдельная страница `/wallet`, а не только оверлей на dashboard
- История операций только на `/wallet` (с dashboard убрана)
- Docker: запуск на PostgreSQL, appuser permissions fix

### Этап 7.6 — Data Migration (SQLite → PostgreSQL)

**Что сделано:**
- Удалены существующие товары/заказы в PostgreSQL
- Перенесены 16 новых пользователей из SQLite (2 перезаписаны)
- Перенесены 30 сервисов (товаров) из SQLite
- Перенесён 1 заказ из SQLite (второй был orphaned — пропущен)
- UUID конвертированы из hex (без дефисов) в стандартный формат PostgreSQL

**Итог:**
| Таблица | Было в PG | Стало |
|---|---|---|
| users | 52 | **68** |
| services | 0 (удалены) | **30** |
| orders | 0 (удалены) | **1** |

### Этап 7.7 — Escrow money flow: transfer + holding account + payout

**Проблема:** при `fund` деньги списывались с покупателя и исчезали из системы;
при `release` продавец не получал оплату.

**Решение:**
1. **`WalletRepository.transfer(from_uid, to_uid, amount)`** — атомарный перевод между
   кошельками двух пользователей: дебет отправителя + кредит получателя + две транзакции.
2. **Escrow holding account** — системный пользователь `escrow@marketplace.local`
   (UUID `1133d650-e7d4-41de-8877-c359682903a4`), создаётся лениво при первом `fund`.
3. **Fund** (`POST /v1/wallet/pay`): buyer → escrow holding (вместо простого списания).
4. **Release** (`POST /v1/escrow/{id}/release` + fallback `PATCH /orders/{id}/status`):
   escrow holding → seller.
5. **Cancel** (`PATCH /orders/{id}/status`): escrow holding → buyer (refund), если заказ
   был в статусе `funded` или `in_progress`.
6. В `TransactionType` добавлен `transfer` для проводок перевода.

**Файлы:**
- `repositories/wallets.py` — `transfer()` + `get_escrow_wallet()`
- `api/v1/wallet.py` — fund использует `transfer()` с `ESCROW_USER_ID`
- `api/v1/escrow_proxy.py` — release переводит escrow → seller
- `api/v1/orders.py` — cancel → refund, release → payout (fallback)
- `models/wallet.py` — добавлен `TransactionType.transfer`
- `tests/test_escrow_money.py` — 11 тестов escrow money flow
- `static/js/escrow-panel.js` — `initWallet()` после release/cancel
- `static/js/dashboard.js` — `initWallet()` после release/cancel
- `templates/dashboard_executor.html` — добавлена `wallet-card.html` + `wallet.js` + `initWallet()`
- `static/js/wallet.js` — убрана привязка к роли `client` (теперь для всех аутентифицированных); добавлен `renderWalletBalanceFallback()`
- `templates/wallet.html`, `templates/dashboard_client.html`, `templates/dashboard_executor.html` — защищённый вызов `if (typeof initWallet === 'function')`

### Phase 7.7b — provider_email в карточках каталога

**Проблема:** в карточках товаров отображалось мок-имя продавца вместо реального email.

**Решение:**
1. **`ServiceRead.provider_email`** — добавлено поле `str | None = None` в Pydantic схему.
2. **Все `GET /services/` эндпоинты** — после `model_validate()` заполняют `provider_email` из `service.provider.email`.
3. **catalog.js** — `s.provider_email || mock.seller` — отображает реальный email, с fallback на мок.

**Файлы:**
- `schemas/services.py` — `provider_email` поле
- `api/v1/services.py` — 3 эндпоинта с `d["provider_email"] = s.provider.email`
- `static/js/catalog.js` — отображение в карточке

### Phase 7.8 — seller_email/buyer_email в деталях заказа

**Проблема:** на странице деталей заказа (`/orders/{id}`) не отображался продавец (seller).

**Решение:**
1. **`OrderRead.seller_email` + `OrderRead.buyer_email`** — добавлены `str | None = None` в Pydantic схему.
2. **`_order_to_dict(order)` helper** — `orders.py`: заполняет email-ы из `order.seller.email` / `order.buyer.email` после валидации Pydantic.
3. **Все 5 эндпоинтов orders** — используют `_order_to_dict()` вместо прямого `model_validate().model_dump()`.
4. **escrow-panel.js** — в `order-info-grid` добавлены строки Seller и Buyer с email-ами.

**Файлы:**
- `schemas/orders.py` — `seller_email`, `buyer_email` поля
- `api/v1/orders.py` — `_order_to_dict()` helper, обновлены все эндпоинты
- `static/js/escrow-panel.js` — отображение в info grid
- `templates/order_detail.html` — cache-bust `escrow-panel.js?v=11`

**Тесты:** без изменений (53/53, новых тестов не требуется — существующие проверяют OrderRead).

### Phase 8 — Полный escrow-цикл через Go (real balances)

**Проблема:** Go-escrow получал `amount="0"` при fund (только Python хранил реальные деньги).
Go не мог корректно обработать cancel (не было CANCELLED статуса/хендлера).

**Решение (Step 1 — Go):**
1. **`StatusCancelled`** + `TxnCancel` в domain — FUNDED/IN_PROGRESS → CANCELLED (терминальный статус).
2. **`Cancel()` в сервисе** — валидация перехода, обнуление balance, CANCEL транзакция, blockchain event.
3. **`HandleCancel()` хендлер** — `POST /:id/cancel` с idempotency middleware.
4. **Полный набор тестов**: domain (8), service (2), handler (3), integration (мок + route).

**Решение (Phases 3–5 — Go test coverage):**
1. **Phase 3 — Middleware + Idempotency tests** (21 тестов): RateLimiter, CORS, RequestID, Logger, Recovery, MiddlewareStack, Idempotency Store + Middleware (no-key/first-pass/cached/5xx-not-cached)
2. **Phase 4 — Repository sqlmock tests** (21 тестов на ветке `step_4`): все 8 методов репозитория с success + error paths (sql.ErrNoRows, rows==0, DB errors)
3. **Phase 5 — Router + Retry queue tests** (15 тестов): router composition (routes, headers, 404, CORS preflight), retry queue (submit, retry-success, queue-full, exhausted, defaults)
4. **Bug fixes**: LoggerMiddleware after RequestIDMiddleware; gin.SetMode перенесён в main.go

**Решение (Step 2 — Python):**
1. **`escrow_client.cancel_escrow()`** — новый метод клиента Python→Go.
2. **`escrow_proxy.fund_escrow`** — исправлен `amount="0"` на `str(order.amount)`.
3. **`escrow_proxy.cancel_proxy`** — новый эндпоинт `POST /{order_id}/cancel` (Go cancel → DB refund).
4. **`orders.update_order_status`** — при cancelled вызывает Go `cancel_escrow` через кэш.
5. **`wallet.pay_order`** — после DB transfer создаёт/fund-ит Go escrow с реальной суммой; graceful degradation при `SERVICE_UNAVAILABLE`; rollback DB при ошибке Go.

**Файлы:**
- `services/go-escrow/internal/domain/escrow.go` — CANCELLED, TxnCancel, validTransitions
- `services/go-escrow/internal/service/escrow_service.go` — Cancel()
- `services/go-escrow/internal/api/handler.go` — HandleCancel()
- `services/go-escrow/internal/api/router.go` — POST /:id/cancel
- `services/python-api/app/services/escrow_client.py` — cancel_escrow()
- `services/python-api/api/v1/escrow_proxy.py` — fund fix, cancel_proxy
- `services/python-api/api/v1/orders.py` — Go cancel on status→cancelled
- `services/python-api/api/v1/wallet.py` — Go fund + rollback
- `services/python-api/models/wallet.py` — escrow_fund_rollback txn_type
- `services/go-escrow/integration_test.go` — cancel route

### Phase 8 — Backend bugfixes: WAL mode, idempotency, escrow safety

**Проблемы:** SQLite table lock contention при последовательных POST→PUT/PUT→GET;
pay_order не-idempotent (повторный вызов падал с 400); complete_escrow вызывал несуществующий Go route.

**Решение:**
1. **WAL mode (`core/db.py`)** — `PRAGMA journal_mode=WAL` + `PRAGMA busy_timeout=5000` на всех SQLite соединениях.
2. **pay_order idempotent (`api/v1/wallet.py`)** — если order уже `funded` и есть успешная `escrow_fund` транзакция, возвращает success вместо ошибки.
3. **`TransactionRepository.get_by_reference()`** — новый метод поиска транзакции по `reference_id` + `type` + опциональный `status`.
4. **complete_escrow → advance_escrow (`api/v1/escrow_proxy.py`)** — заменён вызов `client.complete_escrow()` (нет такого Go route) на `client.advance_escrow(escrow_id, status="COMPLETED")`.
5. **_process_response non-JSON safety (`escrow_client.py`)** — `response.json()` wrapped в try/except ValueError с fallback `{}`.
6. **Fetch timeout (`static/js/api.js`)** — `AbortController` с 15s timeout на всех `apiFetch` вызовах.

**Файлы:**
- `services/python-api/core/db.py` — WAL + busy_timeout
- `services/python-api/api/v1/wallet.py` — idempotent pay_order
- `services/python-api/repositories/wallets.py` — get_by_reference()
- `services/python-api/api/v1/escrow_proxy.py` — complete → advance
- `services/python-api/app/services/escrow_client.py` — non-JSON safety

### Phase 8 — Frontend bugfixes: My Services refresh, pagination, save button

**Проблемы:** После create/delete товара список не обновлялся (только хард-рефреш).
get `/services/my` возвращал только 20 товаров (дефолт лимит). Кнопка Save оставалась disabled после ошибки.

**Решение (Round 1 — pagination + await):**
1. **`apiFetchMyServices(limit, offset)`** — передаёт `?limit=100&offset=0` (все товары, не 20).
2. **`_resetSaveBtn()` helper** — сбрасывает disabled + textContent на Save.
3. **`initMyServices()` возвращает promise** — `return apiFetchMyServices().then(...)`, callers могут `await`.
4. **saveService/deleteService await initMyServices()** — список обновляется до возврата из функции.
5. **saveService catch handler** — закрывает модалку и вызывает `initMyServices()` даже при ошибке.
6. **Cache-buster:** dashboard.js v=12, wallet.js v=5.

**Решение (Round 2 — Cache-Control + async/await + replaceChild):** (не помогло)
1. **Cache-Control: no-cache** во всех fetch запросах (api.js).
2. **_t=timestamp** cache-busting query-param в apiFetchMyServices.
3. **initMyServices переписан** с `.then/.catch` на `async/await` + `try/catch`.
4. **DOM update через replaceChild** — новый `<div id="services-content">` заменяет старый (вместо container.innerHTML).
5. **console.log** — логирование count + IDs из API ответа.
6. **api.js v=8** в base.html; dashboard.js v=14.

**Файлы:**
- `services/python-api/static/js/api.js` — limit/offset, Cache-Control, _t
- `services/python-api/static/js/dashboard.js` — initMyServices async, _resetSaveBtn, await, replaceChild, console.log
- `services/python-api/templates/dashboard_executor.html` — js v=14
- `services/python-api/templates/base.html` — api.js v=8

**Не решено:** create/delete всё ещё не обновляет список — требуется дальнейшая диагностика.

### Phase 8 — Admin dispute management panel

**Проблема:** администратор не мог видеть и разрешать споры (disputed заказы). `admin@marketplace.local` не проходил Pydantic EmailStr валидацию (`.local` домен).

**Решение:**
1. **Admin email fix** — создан рабочий администратор `testadmin@gmail.com / admin123!` в БД напрямую.
2. **Status column fix** — `orders.status` расширен с `varchar(11)` до `varchar(32)` (ALTER TABLE), чтобы `resolved_refund` (14 символов) влезал.
3. **Admin API endpoint** — `api/v1/admin.py`: `GET /admin/disputes` (список споров), `POST /admin/disputes/{id}/release` (выплата продавцу), `POST /admin/disputes/{id}/refund` (возврат покупателю).
4. **Emails in API** — `_order_to_dict()` возвращает `buyer_email` и `seller_email` для отображения в админке.
5. **Dispute reason** — `dispute_escrow` сохраняет reason в `notes`; старые споры заполнены "Disputed by user".
6. **Admin frontend** — `admin_disputes.html` + `admin-disputes.js`: таблица споров, Details модалка (reason, emails, order info), кнопки Release/Refund с подтверждением, рефреш списка после действия, placeholder для блокчейн-секции.

**Файлы:**
- `services/python-api/api/v1/admin.py` — 3 эндпоинта админки
- `services/python-api/templates/admin_disputes.html` — страница управления спорами
- `services/python-api/static/js/admin-disputes.js` — JS логика
- `scripts/seed-admin.ps1` — PowerShell скрипт создания админа
- `scripts/check_disputes.py` — скрипт проверки споров в БД

**API эндпоинты:**
| Метод | Путь | Описание |
|---|---|---|
| GET | `/admin/disputes` | Список disputed заказов с buyer_email/seller_email |
| POST | `/admin/disputes/{id}/release` | Разрешить спор в пользу продавца (→ released) |
| POST | `/admin/disputes/{id}/refund` | Разрешить спор в пользу покупателя (→ resolved_refund) |

### Phase 9 — Integration & inter-service hardening

**Реализовано (без Solidity/Ethereum):**

1. **`blockchain_verified` поле** (`schemas/orders.py`, `api/v1/orders.py`, `api/v1/admin.py`) — добавлено в `OrderRead`. `_order_to_dict()` стал async: для непендинговых заказов вызывает `_check_blockchain_audit()` (HTTP к `blockchain-sim:8082` с in-memory кешем). В тестах `TESTING=1` отключает HTTP.
2. **Dashboard 🔗 колонка** (`static/js/dashboard.js`, `static/css/dashboard.css`, шаблоны) — `blockchainBadge()` отображает ✅ (верифицирован), ❌ (не найден), ⚪ (pending/created — ещё нет записи). Добавлена в `renderClientOrders()`, `renderIncomingOrders()`, `showSkeleton()`.
3. **Escrow panel индикатор** (`static/js/escrow-panel.js`, `dashboard.css`) — `renderBlockchainIndicator()` показывает ✅/❌ бейдж с числом блоков и хешем последнего. Обновляется после каждого escrow-действия. Старая ссылка на `/audit/{id}` убрана.
4. **Исправление парсинга ответа** (`api/v1/orders.py`) — blockchain-sim возвращает плоский JSON-массив на 200, не `{"blocks": [...]}`.
5. **Исправление cancelled** — отменённые заказы показывают реальный статус (✅/❌), а не ⚪.
6. **Fix: новые заказы не появлялись в таблице** (`repositories/orders.py`) — `RepositoryBase.list()` не имел `ORDER BY`, PostgreSQL возвращал строки в неопределённом порядке, и новые заказы могли оказаться на offset>=50 (вне первой страницы). Добавлен `_list_ordered()` с `ORDER BY created_at DESC`.

**Файлы:**
- `services/python-api/schemas/orders.py` — `blockchain_verified: bool = False`
- `services/python-api/api/v1/orders.py` — `_check_blockchain_audit()`, async `_order_to_dict()`
- `services/python-api/api/v1/admin.py` — async `_order_to_dict` с `blockchain_verified`
- `services/python-api/repositories/orders.py` — `_list_ordered()` с сортировкой
- `services/python-api/static/js/dashboard.js` — `blockchainBadge()`, 🔗 колонка
- `services/python-api/static/js/escrow-panel.js` — `renderBlockchainIndicator()`
- `services/python-api/static/css/dashboard.css` — стили `.blockchain-indicator`, `.bc-cell`, `.bc-verified`
- `services/python-api/templates/dashboard_client.html`, `dashboard_executor.html` — `<th>🔗</th>`

**Phase 9 integration fixes:**

1. **`escrow_proxy.py:193`** — `complete_escrow` fallback flag: исправлен с `escrow_id is None` на `go_ok` (отслеживает успешность Go-вызова, а не наличие escrow_id).
2. **`orders.py:44`** — заменён `except Exception: pass` на конкретные исключения (`AsyncHTTPClientError`, `httpx.ConnectError`) + `logger.warning`/`logger.exception` + TTL 5 минут для `_blockchain_cache`.
3. **`blockchain_client.go:85`** — `SubmitEvent` теперь возвращает `nil, nil` при успешной постановке в очередь retry (вместо `fmt.Errorf`). Асинхронный retry не должен считаться ошибкой.
4. **Go tests updated** — `TestSubmitEvent_Timeout`, `TestSubmitEvent_QueueAndRetry`, `TestSubmitEvent_RetrySuccess`: `assert.Error` → `assert.NoError`. `TestSubmitEvent_QueueFull` переписан без race condition. Все 8 SubmitEvent тестов проходят.

**Phase 10 (Financial Accuracy) — audited + skipped:**
- Все суммы уже `Decimal`/`NUMERIC(19,4)` ✅
- Pydantic `condecimal(places=4)` на входе ✅
- Go `.Truncate(4)` в handler.go ✅
- `float` для денег нет в production ✅
- `quantize` — defence-in-depth, конкретного бага не чинит
- **Решение: пропустить** (как Phase 8)

**Ветка:** `step_12` (все фазы 1–10 завершены/пропущены)
```
┌─────────────────────────────────────────────────────────┐
│ Frontend (MetaMask / Web3)                              │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│ Solidity Smart Contracts (Ethereum)                      │
│   EscrowFactory.sol, MarketplaceEscrow.sol,              │
│   DisputeResolver.sol, EscrowToken.sol                   │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│ Go Escrow (on-chain settlement, go-ethereum)             │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│ Python API (Web3.py audit hooks)                         │
└─────────────────────────────────────────────────────────┘
```

### Phase 10 — Category, Description, Discount, Read-only modal, Search, Sort

**Файлы:**
- `models/services.py` — `category` (String, nullable), `discount` (Integer, nullable)
- `schemas/services.py` — `category`, `discount` в ServiceCreate/Update/Read
- `repositories/services.py` — create/update передают category + discount
- `api/v1/services.py` — все эндпоинты передают category + discount
- `alembic/versions/5982cc81659d_add_category_to_services.py` — ADD COLUMN category
- `alembic/versions/b9cc5c4019ff_add_discount_to_services.py` — ADD COLUMN discount (вручную очищена от ложных DROP TABLE)
- `scripts/seed_categories.py` — назначает категории 35 существующим товарам
- `static/js/catalog.js` — `s.category` вместо mock, `s.discount` с fallback, бейдж `-X%`, two prices (original strikethrough + discounted), `auth.role !== 'client'` → readOnly modal
- `static/js/dashboard.js` — svc-category select, svc-discount input, `_allServices` + `filterMyServices()` + `renderFilteredServices()` (поиск по названию); `_sortOrders()` + `toggleClientSort()`/`toggleIncomingSort()` (сортировка заказов); сохранение/восстановление в модалке
- `static/js/order-modal.js` — `readOnly` параметр: блокировка qty+submit для seller/admin, Unit Price строка
- `static/js/api.js` — `apiCreateService(title, ..., discount)` — новый параметр discount
- `static/css/catalog.css` — `.service-card-description` (line-clamp, 80 chars)
- `static/css/main.css` — `.sortable`, `.sort-arrow`, `.sort-arrow.active` (стили сортируемых заголовков)
- `templates/catalog.html` — sidebar category filter `<select>`, description в карточке
- `templates/dashboard_executor.html` — `<select id="svc-category">`, `<input id="svc-discount">`, `<input id="svc-search">`; sortable `<th>` для Amount/Status/Date
- `templates/dashboard_client.html` — sortable `<th>` для Amount/Status/Date
- `templates/order-modal.html` — description row, Unit Price row
- `templates/wallet-card.html` — `<option value="card" disabled>Банковская карта (недоступно)</option>`

**Функциональность:**
1. **Category** — опциональная категория услуги (16 предопределённых опций). Фильтр `?category=` в GET /services/. Сайдбар в каталоге с динамическим `<select>`. Выбор категории при создании/редактировании услуги в dashboard.
2. **Description in card** — краткое описание (до 80 символов, `-webkit-line-clamp: 2`) под названием на карточке каталога.
3. **Description + Unit Price in modal** — описание товара и строка Unit Price (с зачёркнутой оригинальной ценой при скидке) в модалке Place Order.
4. **Discount** — опциональный процент скидки (0–100). В каталоге: бейдж `-X%`, оригинальная цена зачёркнута, discounted цена основная. В order modal: оригинал зачёркнут, discounted для расчёта Total. Fallback на `mock.discount` если `s.discount` не задан.
5. **Read-only modal** — клик по карточке открывает модалку для всех ролей. Client — полный функционал (submit enabled). Seller/Admin — read-only (qty disabled, submit disabled с сообщением "Only clients can place orders"). Кнопка "Add to Cart" для non-client показывает toast-ошибку.
6. **Search My Products** — текстовое поле на дашборде исполнителя, фильтрует услуги по названию в реальном времени (client-side).
7. **Sortable orders** — кликабельные заголовки Amount/Status/Date в таблицах заказов клиента и исполнителя. Первый клик — по возрастанию, второй — по убыванию. Серый `⇅` на неактивных, синий `▲`/`▼` на активной колонке.
8. **Card method unavailable** — в модалке пополнения кошелька способ "Банковская карта" отображается как disabled с текстом "(недоступно)".

**Ветка:** `step_10` (Phase 10); `step_12` (Phase 1–5 verification — CI + fixes + coverage 80%)

## Текущее состояние

**Docker: 5 контейнеров (все healthy)**
| Сервис | Порт | Статус |
|---|---|---|
| marketplace_postgres | 5432 | ✅ healthy |
| marketplace_redis | 6379 | ✅ healthy |
| marketplace_python_api | 8000 | ✅ healthy |
| marketplace_go_escrow | 8081 | ✅ healthy |
| marketplace_blockchain_sim | 8082 | ✅ healthy |

**Frontend:**
- ✅ Catalog page — filter/search/sort/pagination работают; отображается реальный email продавца; категории (фильтр в сайдбаре); описание в карточке (80 chars, line-clamp); скидка (бейдж -X%, two prices)
- ✅ Auth flow — register → login → role-based redirect (client→/dashboard/client, provider→/dashboard/executor); seller/admin видят read-only модалку заказа при клике на карточку
- ✅ Client dashboard — таблица заказов с "Details" → /orders/{id}, карточка баланса с автобновлением после release/cancel; сортировка по Amount/Status/Date (клик на заголовок)
- ✅ Executor dashboard — табы: My Services (CRUD + category + discount поля + поиск по названию) + Incoming Orders (сортировка по Amount/Status/Date), карточка баланса с автобновлением после release/cancel
- ✅ Order detail — escrow panel с 5-шаговым таймлайном + action кнопки (fund/advance/complete/release/dispute/cancel); отображает Seller и Buyer email; описание товара и Unit Price (зачёркнутый оригинал при скидке)
- ✅ Wallet — карточка баланса, пополнение, история транзакций, навигация, автообновление после release/cancel
- ✅ Wallet page (`/wallet`) — отдельная страница с историей транзакций
- ✅ Escrow proxy — `/v1/escrow/:order_id/:action` с Go-first → fallback на PATCH
- ✅ Wallet pay — `POST /v1/wallet/pay` переводит buyer → escrow holding **+ Go fund с реальной суммой**
- ✅ **Go escrow получает реальные средства** — `amount="0"` исправлен на `str(order.amount)` во всех fund-вызовах
- ✅ **Go Cancel handler** — `POST /v1/escrow/{id}/cancel` (FUNDED/IN_PROGRESS → CANCELLED, balance=0, blockchain event)
- ✅ Release payout — escrow holding → seller (через proxy + fallback)
- ✅ Cancel refund — escrow → buyer + **Go cancel_escrow** (если escrow существует)
- ✅ Blockchain audit trail — `/audit/{order_id}` с SHA-256 верификацией
- ✅ Blockchain indicator in escrow panel — ✅/❌ бейдж с числом блоков и хешем, обновляется после действий
- ✅ Dashboard 🔗 column — `blockchainBadge()` показывает ✅/❌/⚪ для каждого заказа
- ✅ `blockchain_verified` поле в API — `OrderRead.blockchain_verified: bool`, заполняется async HTTP к `blockchain-sim:8082`
- ✅ **Fix: новые заказы отображаются** — добавлен `ORDER BY created_at DESC` в `OrderRepository`
- ✅ Cache-busting — `?v=N` на всех CSS/JS
- ✅ Navbar — статический HTML с JS-переключением между гостем и user
- ✅ Footer — copyright на всех страницах, прижат к низу
- ✅ **Admin dispute panel** — `/admin/disputes`: таблица споров, Details модалка, Release/Refund кнопки, buyer/seller email, dispute reason

**Известные проблемы (некритичные для курсовой):**
- ❌ **In-memory cache escrow_id** — `_escrow_cache` теряется при рестарте Python API. Fallback на PATCH отрабатывает корректно. Для production нужен Redis.
- ❌ **No PostgreSQL in Go integration tests** — unit-тесты используют моки sqlmock/httptest. PostgreSQL тесты есть за `postgres_integration` build tag.
- ❌ **Admin `.local` email** — `admin@marketplace.local` не проходит Pydantic EmailStr. Рабочий админ: `testadmin@gmail.com / admin123!`.

**Тесты:**
- Python: 81/81 passed (80% coverage on api/models/repositories/core; coverable target met ✅)
- Go: ~122 тестовых функций, все OK (api ~88.5%, service ~89.4%, repository ~97.3%, domain/config 100%, clients ~91.5%, db ~76.7%, middleware/idempotency ~85%, router ~90%)
- Blockchain: 26/26 passed (99% coverage)
- **Общая оценка покрытия Go: ~80.5%** — все пакеты покрыты

**SAST:**
- bandit: 0 Critical/High
- gosec: 0 issues

**Phase 10 (Financial Accuracy) — audited + skipped:**
- Все суммы уже `Decimal`/`NUMERIC(19,4)` — `quantize` был бы defence-in-depth, конкретного бага нет
- Решение: пропущен, как Phase 8 (UI polish)

## Ключевые конвенции и решения

| Правило | Детали |
|---|---|
| Деньги | `NUMERIC(19,4)` / `Decimal` — никогда float |
| ID | UUID v4 |
| Межсервисное взаимодействие | REST + `X-Request-ID` + `X-Idempotency-Key` |
| Retry | 3 attempts, exponential backoff + jitter |
| Auth | JWT (access + refresh), Bearer header |
| Docker | Multi-stage, distroless, non-root user |
| SAST | bandit + gosec — 0 Critical |
| UI | Pure HTML5/CSS3/Vanilla JS без CDN/фреймворков. FastAPI StaticFiles + Jinja2Templates. Auth+Catalog реализованы |
| LR-маркеры | `# LR #N` в комментариях для академической отслеживаемости |

## Важные архитектурные заметки

1. **`context.Background()` в async goroutines** — 4 вызова `emitBlockchainEvent` должны пережить triggering HTTP request
2. **`math/rand` для jitter** (с #nosec G404) — timing jitter не криптографический
3. **`GO_ESCROW_BASE_URL`** дефолт: `http://go-escrow:8081` (Docker internal) в python-api
4. **`BLOCKCHAIN_SIM_URL`** дефолт: `http://blockchain-sim:8082` (Docker internal) в go-escrow
5. **`/v1/escrow/{id}/advance`** — добавлен, потому что Release/Dispute требуют COMPLETED, но не было пути FUNDED→IN_PROGRESS→COMPLETED
6. **bcrypt==4.2.1** — зафиксирован, passlib несовместим с bcrypt>=5
7. **`/api/static/`** — статика отдаётся через `/api/static/...` из-за `root_path="/api"` в `core/config.py`
8. **Client-side filtering** — `/v1/services/` не поддерживает search/sort/price filter, вся логика на JS
9. **`apiFetchServices({ limit: 100 })`** — на каталоге запрашиваются все published услуги (max 100) для клиентской фильтрации
10. **Service title in orders** — OrderRead не содержит `service_title`, поэтому dashboard подгружает его через `GET /v1/services/:id` с кешированием (N+1 проблема). В будущем добавить `service_title` в OrderRead.
11. **Index-based inline onclick** — для XSS-безопасности данные в onclick передаются через числовой индекс массива `serviceData[]`, а не строкой с экранированием.
12. **Python 3.14 + pydantic** — `pydantic>=2.11.2` (unpinned) необходим для совместимости с Python 3.14, т.к. pydantic-core <2.46.x не поддерживает новую версию.
13. **Escrow proxy** — создан отдельный роутер `api/v1/escrow_proxy.py`, а не изменения существующих эндпоинтов. Стратегия: Go escrow → fallback на PATCH статуса заказа при недоступности Go.
14. **In-memory cache escrow_id** — `_escrow_cache: dict[str, str]` хранит `order_id→escrow_id`. Теряется при рестарте Python API (достаточно для dev, в production нужен Redis).
15. **Escrow fallback в JS** — при `e.status === 404 || e.status === 0` (Go не доступен) JS вызывает `apiUpdateOrderStatus()` напрямую, минуя escrow-поток.
16. **Cache-busting через `?v=N`** — версионирование CSS/JS через query-параметр, без изменения имён файлов. Инкрементировать при изменении статики.
17. **Logout на `/`** — после выхода пользователь видит landing page, а не `/login`, что улучшает UX.
18. **Wallet/pay (escrow fund)** — `POST /v1/wallet/pay` переводит сумму с кошелька покупателя на escrow holding account (системный пользователь `escrow@marketplace.local`). Go-escrow НЕ вызывается (все средства в Python БД).
19. **Escrow release выплачивает продавцу** — при `release` (через `POST /v1/escrow/{id}/release` или fallback `PATCH /orders/{id}/status`) деньги переводятся с escrow holding на кошелёк продавца через `WalletRepository.transfer()`.
20. **`WalletRepository.transfer(from_uid, to_uid, amount)`** — атомарный перевод: дебет отправителя + кредит получателя + две транзакции. Используется для escrow fund (buyer→escrow), release (escrow→seller) и cancel refund (escrow→buyer).
21. **Блокировка роли в initWallet()** — убрана: теперь баланс загружается для всех аутентифицированных пользователей, а не только `client`.
22. **Wallet на дашборде исполнителя** — добавлен `wallet-card.html` + `initWallet()` на `/dashboard/executor`, чтобы провайдер видел баланс и пополнения.
23. **`seller_email` / `buyer_email` в OrderRead** — Pydantic-поля `str | None = None`, заполняются через `_order_to_dict()` helper из SQLAlchemy relationship `order.seller.email` / `order.buyer.email`. `from_attributes` не может напрямую читать `order.seller.email`, поэтому используется пост-валидационная вставка — та же техника, что и `provider_email` в ServiceRead.
24. **`SubmitEvent` async retry** — при ошибке HTTP событие ставится в асинхронную очередь goroutine, а `SubmitEvent` возвращает `nil, nil` (не ошибку). Успешная постановка в очередь не считается ошибкой.
25. **Phase 8 (UI polish) и Phase 10 (quantize) пропущены** — первая не критична (всё уже работает), вторая — defence-in-depth (нет деления/умножения денежных сумм).

## Команды для быстрого старта

```bash
make up              # docker compose up -d --build (все сервисы)
make down            # docker compose down --volumes (стереть БД)
make test            # Python + Go тесты
make test-python     # pytest --cov --lcov
make test-go         # go test -v -cover
make test-all        # test + coverage-report
make lint-python     # ruff check
make lint-go         # go vet
make sast            # bandit + gosec
make ai-log          # напоминание добавить запись в ai-usage-log.md

# Pytest напрямую
cd services/python-api && python -m pytest tests/ -v
cd services/python-api && python -m pytest tests/ -v --cov=api --cov=core --cov=models --cov=repositories --cov=app --cov-report=term-missing

# Go test напрямую
cd services/go-escrow && go test ./... -v -count=1

# Blockchain test напрямую
cd services/blockchain-sim && python -m pytest tests/ -v

# Ручная проверка
curl http://localhost:8000/health
curl http://localhost:8081/health
curl http://localhost:8082/health

# Frontend
curl http://localhost:8000/                          # Landing
curl http://localhost:8000/login                      # Login page (200)
curl http://localhost:8000/register                   # Register page (200)
curl http://localhost:8000/catalog                    # Catalog page (200)
curl http://localhost:8000/dashboard/client           # Client dashboard (200)
curl http://localhost:8000/dashboard/executor         # Executor dashboard (200)
curl http://localhost:8000/orders/                    # Orders list (200)
curl http://localhost:8000/admin/disputes              # Admin dispute panel (200)
curl http://localhost:8000/api/static/css/main.css    # CSS (200)
curl http://localhost:8000/api/static/js/api.js       # JS (200)
curl http://localhost:8000/api/static/js/dashboard.js # Dashboard JS (200)
curl http://localhost:8000/api/v1/services/           # API services (JSON)

# Admin
curl -X POST http://localhost:8000/auth/login -H "Content-Type: application/json" -d '{"email":"testadmin@gmail.com","password":"admin123!"}'
curl http://localhost:8000/admin/disputes -H "Authorization: Bearer <token>"
curl -X POST http://localhost:8000/admin/disputes/{id}/release -H "Authorization: Bearer <token>"
curl -X POST http://localhost:8000/admin/disputes/{id}/refund -H "Authorization: Bearer <token>"
```
