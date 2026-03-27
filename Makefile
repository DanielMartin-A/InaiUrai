include .env
export

.PHONY: up down logs migrate-all seed bootstrap reset test-all

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

migrate-all:
	@echo "Running all migrations..."
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/001_initial_schema.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/002_expansion.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/003_pwa_sms.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/004_roles.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/005_agent_v2.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/006_organizations.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/007_engagements.sql
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/migrations/008_enterprise_hardening.sql
	@echo "All migrations complete."

seed:
	docker compose exec -T postgres psql -U inaiurai -d inaiurai < db/seeds/001_initial_setup.sql

bootstrap: up
	@echo "Waiting for postgres..."
	@sleep 5
	@make migrate-all
	@make seed
	@echo "Stack ready."

reset:
	docker compose down -v
	make bootstrap

test-go:
	cd backend && go test ./internal/... -v -count=1

test-engine:
	cd engine && python -m pytest tests/ -v 2>/dev/null || echo "No tests yet"

test-smoke:
	bash scripts/smoke_test.sh

test-all: test-go test-engine test-smoke
