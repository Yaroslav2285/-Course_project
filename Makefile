# LR #11: Containerization
# LR #1: Git/CI
.PHONY: up down lint test test-python test-go build ai-log

up:
	docker compose up -d --build

down:
	docker compose down --volumes

lint:
	pre-commit run --all-files

test: test-python test-go

test-python:
	cd services/python-api && pytest -v --cov --cov-report=term --cov-report=lcov:coverage/lcov.info --junitxml=coverage/junit.xml

test-go:
	cd services/go-escrow && go test ./... -v -cover -coverprofile=cover.out

test-all: test-python test-go coverage-report

coverage-report:
	@echo "Coverage reports generated:"
	@echo "  Python: services/python-api/coverage/lcov.info"
	@echo "  Go:     services/go-escrow/cover.out"

lint-python:
	cd services/python-api && ruff check .

lint-go:
	cd services/go-escrow && go vet ./...

sast-python:
	cd services/python-api && bandit -r . -f json -o bandit-report.json 2>/dev/null || echo "bandit done"

sast-go:
	cd services/go-escrow && gosec -fmt json -out gosec-report.json ./... 2>/dev/null || echo "gosec done"

sast: sast-python sast-go

build:
	docker compose build

ai-log:
	@echo "Append AI usage entry to docs/ai-usage-log.md"

.PHONY: up down lint test test-python test-go test-all coverage-report lint-python lint-go sast-python sast-go sast build ai-log
