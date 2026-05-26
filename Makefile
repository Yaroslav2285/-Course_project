# LR #11: Containerization
# LR #1: Git/CI
.PHONY: up down lint test build ai-log

up:
	docker compose up -d --build

own:
	docker compose down --volumes

down:
	docker compose down --volumes

lint:
	pre-commit run --all-files

test:
	@echo "No service tests implemented yet. Add pytest and go test targets later."

build:
	docker compose build

ai-log:
	@echo "Append AI usage entry to docs/ai-usage-log.md"
