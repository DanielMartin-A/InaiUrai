.PHONY: up down migrate seed-db seed bootstrap test build \
       test-go test-agents test-frontend test-smoke test-all

DB_URL    = postgres://inaiurai_dev:devpassword@localhost:5432/inaiurai?sslmode=disable
DB_DOCKER = postgres://inaiurai_dev:devpassword@postgres:5432/inaiurai?sslmode=disable

# ---------------------------------------------------------------------------
# Docker Compose
# ---------------------------------------------------------------------------

up:
	docker compose up -d --build

down:
	docker compose down

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

migrate:
	migrate -path db/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_URL)" down

seed-db:
	docker cp db/seeds/001_platform_bootstrap.sql inaiurai-db:/tmp/seed.sql
	docker exec -e PGPASSWORD=devpassword inaiurai-db psql -U inaiurai_dev -d inaiurai -v ON_ERROR_STOP=1 -f /tmp/seed.sql

# ---------------------------------------------------------------------------
# Agents
# ---------------------------------------------------------------------------

seed:
	DATABASE_URL="$(DB_URL)" python3 agents/register_agents.py

# ---------------------------------------------------------------------------
# Full Bootstrap (one-shot setup)
# ---------------------------------------------------------------------------

bootstrap: up
	@echo "⏳ Waiting for services to be healthy..."
	@sleep 5
	$(MAKE) migrate
	$(MAKE) seed-db
	$(MAKE) seed
	@echo "✅ Stack is ready.  Frontend → http://localhost:3000  Backend → http://localhost:8080"

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

test-go:
	cd backend && go test ./internal/... -v

test-agents:
	cd agents && python3 -m pytest tests/ -v

test-frontend:
	cd frontend && npx vitest run

test-smoke:
	bash scripts/smoke_test.sh

test-all: test-go test-agents test-frontend
	@bash scripts/validate_schemas.sh
	@echo ""
	@echo "All unit/integration tests passed."

test: test-smoke

# ---------------------------------------------------------------------------
# Build (local, without Docker)
# ---------------------------------------------------------------------------

build:
	@echo "Building Go backend..."
	cd backend && go build -o bin/api cmd/api/main.go
	@echo "Building Next.js frontend..."
	cd frontend && npm run build
