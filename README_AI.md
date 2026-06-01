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

**Тесты:** 58 test functions: domain (50+), service (9), handler (15), clients (5), postgres integration (3), top-level integration (2). **Покрытие по пакетам:** domain ~95% (отлично), service ~55%, handler ~50%, clients ~45%, repository ~5%, middleware/idempotency/router/config/db/main ~0%. **Общая оценка: ~30-40%**

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
- Комментарии: `# LR #6`, `# LR #10`, `# LR #12`

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
- ✅ Catalog page — filter/search/sort/pagination работают; отображается реальный email продавца
- ✅ Auth flow — register → login → role-based redirect (client→/dashboard/client, provider→/dashboard/executor)
- ✅ Client dashboard — таблица заказов с "Details" → /orders/{id}, карточка баланса с автобновлением после release/cancel
- ✅ Executor dashboard — табы: My Services (CRUD) + Incoming Orders, карточка баланса с автобновлением после release/cancel
- ✅ Order detail — escrow panel с 5-шаговым таймлайном + action кнопки (fund/advance/complete/release/dispute/cancel); отображает Seller и Buyer email
- ✅ Wallet — карточка баланса, пополнение, история транзакций, навигация, автообновление после release/cancel
- ✅ Wallet page (`/wallet`) — отдельная страница с историей транзакций
- ✅ Escrow proxy — `/v1/escrow/:order_id/:action` с Go-first → fallback на PATCH
- ✅ Wallet pay — `POST /v1/wallet/pay` переводит buyer → escrow holding **+ Go fund с реальной суммой**
- ✅ **Go escrow получает реальные средства** — `amount="0"` исправлен на `str(order.amount)` во всех fund-вызовах
- ✅ **Go Cancel handler** — `POST /v1/escrow/{id}/cancel` (FUNDED/IN_PROGRESS → CANCELLED, balance=0, blockchain event)
- ✅ Release payout — escrow holding → seller (через proxy + fallback)
- ✅ Cancel refund — escrow → buyer + **Go cancel_escrow** (если escrow существует)
- ✅ Blockchain audit trail — `/audit/{order_id}` с SHA-256 верификацией
- ✅ Cache-busting — `?v=N` на всех CSS/JS
- ✅ Navbar — статический HTML с JS-переключением между гостем и user
- ✅ Footer — copyright на всех страницах, прижат к низу

**Известные проблемы:**
- ❌ **In-memory cache escrow_id** — `_escrow_cache` теряется при рестарте Python API. Для production нужен Redis. (Addresses via Redis cache in Phase 8+)
- ❌ **No PostgreSQL in integration tests** — Go integration test использует моки, не реальную БД. (Built-tag-guarded PostgreSQL tests exist but require `TEST_DB_DSN`)
- ❌ **Go test coverage gaps (~30-40%)** — middleware, idempotency, router, config, db, blockchain events, retry queue, и бóльшая часть error paths не покрыты тестами

**Тесты:**
- Python: 54/54 passed
- Go: 58 тестовых функций, все OK (domain ~95%, service ~55%, handler ~50%, clients ~45%, repository ~5%, middleware+idempotency+router+config+db+main ~0%)
- Blockchain: 26/26 passed (99% coverage)
- **Общая оценка покрытия Go: ~30-40%** — домен отлично, инфраструктура не покрыта

**SAST:**
- bandit: 0 Critical/High
- gosec: 0 issues

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
curl http://localhost:8000/api/static/css/main.css    # CSS (200)
curl http://localhost:8000/api/static/js/api.js       # JS (200)
curl http://localhost:8000/api/static/js/dashboard.js # Dashboard JS (200)
curl http://localhost:8000/api/v1/services/           # API services (JSON)
```
