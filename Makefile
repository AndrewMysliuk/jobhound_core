# jobhound_core — personal job aggregator (collect → filter → score → storage → bot)

.PHONY: help build build-worker build-migrate run run-worker test test-integration fmt vet tidy migrate-up migrate-down migrate-version

help:
	@echo "Targets: build | build-worker | build-migrate | run | run-worker | test | test-integration | fmt | vet | tidy | migrate-up | migrate-down | migrate-version"

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
