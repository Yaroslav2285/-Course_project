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
	cd services/python-api && pytest -v --cov --cov-report=term

test-go:
	cd services/go-escrow && go test ./... -v -cover

build:
	docker compose build

ai-log:
	@echo "Append AI usage entry to docs/ai-usage-log.md"
