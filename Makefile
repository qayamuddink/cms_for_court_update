.PHONY: all run test lint lint-openapi migrate-up migrate-down migrate-status seed docker-up docker-down check help

DATABASE_URL ?= postgres://livelaw:livelaw_secret@localhost:5432/livelaw?sslmode=disable
OPENAPI_SPEC ?= contracts/openapi/openapi.yaml

all: check

run:
	@echo "Starting backend API server..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

test:
	@echo "Running backend test suite..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go test -v -race ./...

lint-openapi:
	@echo "Validating OpenAPI specification..."
	cd backend && go run ./cmd/openapi-lint -spec $(OPENAPI_SPEC)

lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd backend && golangci-lint run ./...; \
	else \
		echo "golangci-lint is not installed locally. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

migrate-up:
	@echo "Applying database migrations..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	@echo "Rolling back last database migration..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down

migrate-status:
	@echo "Checking migration status..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate status

seed:
	@echo "Seeding initial accounts (one per role)..."
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/seed

docker-up:
	@echo "Starting full Docker Compose environment..."
	docker compose up -d --build

docker-down:
	@echo "Stopping Docker Compose environment..."
	docker compose down -v

docker-logs:
	docker compose logs -f

check: lint-openapi test
	@echo "All Phase 1 quality checks passed."

help:
	@echo "LiveLaw Platform Makefile Targets:"
	@echo "  make run             - Start the Go backend locally"
	@echo "  make test            - Run all unit and integration tests"
	@echo "  make lint-openapi    - Validate the OpenAPI 3 specification"
	@echo "  make lint            - Run golangci-lint"
	@echo "  make migrate-up      - Run all pending migrations"
	@echo "  make migrate-down    - Roll back the last migration"
	@echo "  make migrate-status  - View applied and pending migrations"
	@echo "  make docker-up       - Build and launch all services with docker compose"
	@echo "  make docker-down     - Stop containers and remove volumes"
	@echo "  make check           - Run lint-openapi and test"
