# jobhound_core — personal job aggregator (collect → filter → score → storage → bot)

.PHONY: help build build-worker build-migrate run run-worker test test-integration fmt vet tidy migrate-up migrate-down migrate-version

help:
	@echo "Targets: build | build-worker | build-migrate | run | run-worker | test | test-integration | fmt | vet | tidy"
	@echo "  build / build-worker — bin/agent (+ bin/worker for build); worker-only: make build-worker"
	@echo "  run-worker         — build and run Temporal worker (JOBHOUND_TEMPORAL_ADDRESS, e.g. docker compose up)"
	@echo "  test              — go test ./... (integration tests need -tags=integration, see test-integration)"
	@echo "  test-integration  — go test -tags=integration ./... (Postgres: JOBHOUND_DATABASE_URL; Temporal: JOBHOUND_TEMPORAL_ADDRESS + running worker)"
	@echo "Migrations (require JOBHOUND_DATABASE_URL or JOBHOUND_MIGRATE_DATABASE_URL):"
	@echo "  migrate-up       — build bin/migrate and apply all pending SQL migrations"
	@echo "  migrate-down     — build bin/migrate and apply one down step"
	@echo "  migrate-version  — print current migration version"
	@echo ""
	@echo "Environment (PostgreSQL / migrations) — names only; see contract for semantics:"
	@echo "  JOBHOUND_DATABASE_URL              — Postgres URL (required once DB is wired)"
	@echo "  JOBHOUND_MIGRATE_DATABASE_URL      — optional migrate-only DSN override"
	@echo "  JOBHOUND_DB_MAX_OPEN_CONNS         — optional pool: max open conns"
	@echo "  JOBHOUND_DB_MAX_IDLE_CONNS         — optional pool: max idle conns"
	@echo "  JOBHOUND_DB_CONN_MAX_LIFETIME_SEC  — optional pool: conn max lifetime (seconds)"
	@echo "Full contract: specs/002-postgres-gorm-migrations/contracts/environment.md"
	@echo ""
	@echo "Environment (Temporal) — names only; see contract for semantics:"
	@echo "  JOBHOUND_TEMPORAL_ADDRESS    — gRPC frontend host:port (required for worker / real client)"
	@echo "  JOBHOUND_TEMPORAL_NAMESPACE  — optional; default namespace: default"
	@echo "  JOBHOUND_TEMPORAL_TASK_QUEUE — optional; default jobhound"
	@echo "Full contract: specs/003-temporal-orchestration/contracts/environment.md"

build: build-worker
	go build -o bin/agent ./cmd/agent

build-worker:
	go build -o bin/worker ./cmd/worker

build-migrate:
	go build -o bin/migrate ./cmd/migrate

migrate-up: build-migrate
	./bin/migrate up

migrate-down: build-migrate
	./bin/migrate down

migrate-version: build-migrate
	./bin/migrate version

run: build
	./bin/agent

# Temporal worker: requires JOBHOUND_TEMPORAL_ADDRESS (e.g. localhost:7233 with docker compose up).
run-worker: build-worker
	./bin/worker

test:
	go test ./...

# Docker-backed tests (build tag integration); see internal/platform/pgsql/migrations_integration_test.go.
test-integration:
	go test -tags=integration ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
