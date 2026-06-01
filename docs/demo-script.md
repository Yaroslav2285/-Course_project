# Сценарий демонстрации (пояснительная записка)

## Запуск проекта

```bash
# 1. Запустить все сервисы
docker-compose up -d

# Проверка, что все 5 контейнеров healthy
docker-compose ps

# 2. Инициализация тестовых данных
cd services/python-api && python scripts/seed_db.py
```

---

## Шаг 1 — Показать инфраструктуру

### 1.1 Docker-сервисы
Показать `docker-compose.yml` — 5 сервисов:
- **postgres** (БД)
- **redis** (кеширование)
- **python-api** (FastAPI, порт 8000)
- **go-escrow** (Gin, порт 8081)
- **blockchain-sim** (FastAPI, порт 8082)

Команда: `docker-compose ps` — все `healthy`.

### 1.2 CI/CD
Показать `.github/workflows/ci.yml` — три джобы:
- **lint**: ruff, gofmt, goimports
- **test**: pytest (81 тестов), go test (116 тестов)
- **sast**: bandit (0 High/Medium), gosec (0 issues)

### 1.3 Makefile
Показать цели: `make up`, `make down`, `make test`, `make lint`, `make sast`, `make build`.

---

## Шаг 2 — Python API (FastAPI + SQLAlchemy)

### 2.1 Регистрация и аутентификация
```bash
# Регистрация клиента
curl -s -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"client@test.com","password":"pass123","role":"client","name":"Client User"}' | jq .

# Регистрация исполнителя
curl -s -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"executor@test.com","password":"pass123","role":"executor","name":"Executor User"}' | jq .

# Регистрация администратора
curl -s -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"pass123","role":"admin","name":"Admin User"}' | jq .

# Логин (получить access + refresh токены)
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"client@test.com","password":"pass123"}' | jq -r '.data.access_token')
echo "TOKEN=$TOKEN"
```

**Что показать:**
- JWT access + refresh токены
- Роли: client, executor, admin
- Хеширование паролей (bcrypt) в `core/security.py`

### 2.2 Каталог услуг
```bash
# Список услуг с пагинацией
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8000/api/v1/services/?limit=10&offset=0" | jq .

# Фильтрация по категории
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8000/api/v1/services/?category=programming" | jq .

# Поиск по названию
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8000/api/v1/services/?search=python" | jq .
```

**Что показать:**
- Пагинация (limit, offset)
- Фильтрация и поиск
- Pydantic схемы валидации

### 2.3 Оформление заказа
```bash
# Создать заказ на услугу (от клиента)
ORDER_ID=$(curl -s -X POST http://localhost:8000/api/v1/orders/ \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"service_id":"<UUID_услуги>"}' | jq -r '.data.id')
echo "ORDER_ID=$ORDER_ID"
```

### 2.4 Wallet и пополнение баланса
```bash
# Пополнить кошелек
curl -s -X POST http://localhost:8000/api/v1/wallet/topup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount":"500.0000"}' | jq .

# Показать баланс
curl -s http://localhost:8000/api/v1/wallet/balance \
  -H "Authorization: Bearer $TOKEN" | jq .

# История транзакций
curl -s "http://localhost:8000/api/v1/wallet/transactions?limit=5" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Что показать:**
- Decimal/NUMERIC(19,4) — 4 знака после запятой
- Все суммы через `Decimal`, никаких `float`
- Явное округление `quantize`

---

## Шаг 3 — Go Escrow (State-machine)

### 3.1 Создание escrow-счета
```bash
# Создать escrow (через Python API → Go)
ESCROW_ID=$(curl -s -X POST http://localhost:8000/api/v1/escrow/ \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"order_id\":\"$ORDER_ID\"}" | jq -r '.data.id')
echo "ESCROW_ID=$ESCROW_ID"

# Статус: CREATED
```

### 3.2 Жизненный цикл (state-machine)
Показать файл `internal/domain/escrow.go` со статусами:
```
CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED
               ↘ CANCELLED
               ↘ DISPUTED → RESOLVED
```

```bash
# Fund — перевести средства на escrow
curl -s -X POST "http://localhost:8000/api/v1/escrow/$ESCROW_ID/fund" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount":"100.0000"}' | jq .
# Статус: FUNDED

# Release — подтвердить выполнение
curl -s -X POST "http://localhost:8000/api/v1/escrow/$ESCROW_ID/release" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Статус: RELEASED — деньги ушли исполнителю
```

**Что показать:**
- Валидация переходов (`IsValidTransition`)
- X-Idempotency-Key (защита от дублирования)
- Rate-limiter на `/fund`

### 3.3 Dispute — спор
```bash
# Создать escrow, залить деньги, открыть спор
curl -s -X POST "http://localhost:8000/api/v1/escrow/$ESCROW_ID/dispute" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"Executor не выполнил работу"}' | jq .
# Статус: DISPUTED

# Админ разрешает спор
curl -s -X PATCH "http://localhost:8000/api/v1/admin/disputes/$DISPUTE_ID/resolve" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resolution":"refund"}' | jq .
# Статус: RESOLVED, деньги возвращены
```

---

## Шаг 4 — Blockchain Simulator (SHA-256)

### 4.1 Цепочка блоков
Показать `blockchain-sim/blockchain.py`:
- SHA-256 хеширование
- Каждый блок хранит hash предыдущего
- SQLite для персистентности

```bash
# Отправить событие (через Go → Blockchain)
# submit автоматически при fund/release/dispute

# Проверить блокчейн
curl -s "http://localhost:8000/api/v1/chain/audit/$ESCROW_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Верификация цепочки
curl -s "http://localhost:8000/api/v1/chain/verify/$ESCROW_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Что показать:**
- SHA-256 хеш-цепочка
- Prepared statements в SQLite
- Невозможность подделать историю

---

## Шаг 5 — UI/Frontend

### 5.1 Аутентификация
Открыть `http://localhost:8000/login`:
- Форма логина/регистрации
- JWT в localStorage
- Role-based redirect (client → catalog, admin → disputes)

### 5.2 Каталог услуг
Открыть `http://localhost:8000/catalog`:
- Поиск, фильтры, пагинация
- Категории и скидки
- Оформление заказа в модалке

### 5.3 Dashboard клиента
Открыть `http://localhost:8000/dashboard`:
- Список заказов с сортировкой
- Статус escrow
- Индикатор blockchain-верификации

### 5.4 Dashboard исполнителя
Открыть `http://localhost:8000/dashboard`:
- CRUD услуг
- Управление заказами
- Просмотр баланса

### 5.5 Wallet
Открыть `http://localhost:8000/wallet`:
- Баланс (Decimal с 4 знаками)
- Пополнение
- История транзакций

### 5.6 Админ-панель
Открыть `http://localhost:8000/admin/disputes`:
- Список споров
- Разрешение (refund/release)
- Blocked-пользователи

---

## Шаг 6 — Интеграция сервисов

### 6.1 Python → Go (escrow_client.py)
Показать `app/services/escrow_client.py`:
- HTTP-запрос с X-Request-ID
- Retry при недоступности
- Fallback: если Go недоступен → прямой PATCH в БД

### 6.2 Go → Blockchain (blockchain_client.go)
Показать `internal/clients/blockchain_client.go`:
- Асинхронная отправка (goroutine)
- Retry с экспоненциальной задержкой
- Очередь недоставленных событий

### 6.3 Полный escrow-цикл
```
topup → fund → release → payout
клиент    escrow   escrow   исполнитель
```

---

## Шаг 7 — Безопасность (SAST)

### 7.1 Bandit (Python)
```bash
cd services/python-api && bandit -r . -f json -o bandit-report.json
# Результат: 0 HIGH, 0 MEDIUM, 191 LOW (assert в тестах)
```

### 7.2 Gosec (Go)
```bash
cd services/go-escrow && gosec -fmt json -out gosec-report.json ./...
# Результат: 0 issues
```

### 7.3 Non-root Docker
Показать в Dockerfile:
- `USER appuser` (Python API, blockchain-sim)
- `gcr.io/distroless/base-debian12:nonroot` (Go escrow)

### 7.4 Prepared statements
Показать в Go: `internal/repository/escrow_repo.go` — все запросы через `$1, $2, ...`
Показать в Python: SQLAlchemy ORM — безопасные параметризованные запросы

---

## Шаг 8 — Тестирование

### 8.1 Python тесты (81 тест, 80% coverage)
```bash
cd services/python-api && pytest --cov=app --cov-report=term --tb=short
```

Показать ключевые тесты:
- `tests/test_auth.py` — регистрация, логин, refresh
- `tests/test_orders.py` — CRUD заказов
- `tests/test_services.py` — фильтры, категории
- `tests/test_wallet.py` — баланс, topup, транзакции
- `tests/test_escrow_money.py` — полный финансовый цикл

### 8.2 Go тесты (~150 тестов, 80.5% coverage)
```bash
cd services/go-escrow && go test -coverprofile=cover.out ./...
```

Показать coverage по пакетам:
- `domain`: 100%
- `config`: 100%
- `repository`: 97.3%
- `service`: 89.4%
- `api`: 88.5%
- `clients`: 91.5%
- `db`: 76.7%

### 8.3 Blockchain sim тесты (26 тестов, 99% coverage)
```bash
cd services/blockchain-sim && pytest --cov=app --cov-report=term
```

---

## Чек-лист проверки (для комиссии)

| № | Проверка | Статус |
|---|----------|--------|
| 1 | Git Flow (master + step_*) | ✅ 8 коммитов в step_12 |
| 2 | Conventional Commits | ✅ `Phase N:`, `Fix:`, `Feat:` |
| 3 | CI/CD (GitHub Actions) | ✅ lint, test, sast |
| 4 | Docker Compose (5 сервисов) | ✅ все healthy |
| 5 | Makefile цели | ✅ up, down, test, lint, sast, build |
| 6 | Python API (FastAPI async) | ✅ 15 эндпоинтов, /v1/ |
| 7 | JWT auth (access+refresh) | ✅ 3 роли |
| 8 | Decimal/NUMERIC(19,4) | ✅ нет float |
| 9 | Go state-machine | ✅ 7 статусов |
| 10 | Prepared statements | ✅ Python ORM + Go $N |
| 11 | Blockchain SHA-256 | ✅ SQLite + верификация |
| 12 | Non-root Docker | ✅ все 3 сервиса |
| 13 | SAST (bandit + gosec) | ✅ 0 Critical/High |
| 14 | Python coverage ≥80% | ✅ 80% |
| 15 | Go coverage ≥80% | ✅ 80.5% |
| 16 | UI (каталог, dashboard, wallet) | ✅ responsive, ARIA |

---

## Команды для быстрой проверки

```bash
# Инфраструктура
docker-compose ps
make test
make sast

# Python API
curl http://localhost:8000/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"email":"testadmin@gmail.com","password":"admin123"}'

# Go escrow
curl http://localhost:8081/health

# Blockchain
curl http://localhost:8082/health

# Coverage
cd services/python-api && pytest --cov=app --cov-report=term --tb=short
cd services/go-escrow && go test -coverprofile=cover.out ./...
cd services/blockchain-sim && pytest --cov=app --cov-report=term
```
