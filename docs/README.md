# Marketplace Services with Escrow

Документация для проекта "Маркетплейс услуг с эскароу-системой".

## Архитектура

Проект состоит из 5 Docker-сервисов:

| Сервис | Технологии | Порт | Описание |
|--------|-----------|------|----------|
| **postgres** | PostgreSQL 15 | 5432 | Основная БД (Python + Go) |
| **redis** | Redis 7 | 6379 | Кеширование (зарезервировано) |
| **python-api** | FastAPI, SQLAlchemy async | 8000 | Auth, каталог, заказы, wallet, UI |
| **go-escrow** | Gin, state-machine | 8081 | Escrow-счета, dispute, blockchain events |
| **blockchain-sim** | FastAPI, SQLite, SHA-256 | 8082 | Симулятор блокчейна |

Взаимодействие: `Python API → Go Escrow → Blockchain Sim` (асинхронно через goroutine).

## Документы

- [`contracts/escrow-api.md`](contracts/escrow-api.md) — REST контракт Python API ↔ Go Escrow
- [`contracts/blockchain-events.md`](contracts/blockchain-events.md) — REST контракт Go Escrow ↔ Blockchain Sim
- [`demo-script.md`](demo-script.md) — сценарий защиты (пошаговая демонстрация)
- [`ai-usage-log.md`](ai-usage-log.md) — журнал изменений

## Быстрый старт

```bash
# Запуск всех сервисов
docker-compose up -d

# Инициализация тестовых данных
cd services/python-api && python scripts/seed_db.py

# Проверка
curl http://localhost:8081/health   # Go escrow
curl http://localhost:8082/health   # Blockchain sim
```

## Покрытие тестами

- **Python API**: 81 тест, 80% coverage
- **Go Escrow**: ~150 тестов, 80.5% coverage
- **Blockchain Sim**: 26 тестов, 99% coverage
