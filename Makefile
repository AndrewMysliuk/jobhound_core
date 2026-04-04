# jobhound_core — personal job aggregator (collect → filter → score → storage → bot)

# Local debug HTTP for collectors (see JOBHOUND_DEBUG_HTTP_ADDR in internal/config).
DEBUG_HTTP_ADDR ?= 127.0.0.1:8080

.PHONY: help build build-worker build-api build-migrate build-retention run run-debug run-worker test test-integration fmt vet tidy migrate-up migrate-down migrate-version

help:
	@echo "Targets: build | build-worker | build-api | build-migrate | build-retention | run | run-debug | run-worker | test | test-integration | fmt | vet | tidy | migrate-up | migrate-down | migrate-version"

build: build-worker build-api
	go build -o bin/agent ./cmd/agent

build-api:
	go build -o bin/api ./cmd/api

build-worker:
	go build -o bin/worker ./cmd/worker

build-migrate:
	go build -o bin/migrate ./cmd/migrate

build-retention:
	go build -o bin/retention ./cmd/retention

migrate-up: build-migrate
	./bin/migrate up

migrate-down: build-migrate
	./bin/migrate down

migrate-version: build-migrate
	./bin/migrate version

run: build
	./bin/agent

# Agent with debug HTTP only (Postman: GET /health, POST /debug/collectors/europe_remotely, POST /debug/collectors/working_nomads). Override: make run-debug DEBUG_HTTP_ADDR=127.0.0.1:9090
run-debug: build
	JOBHOUND_DEBUG_HTTP_ADDR=$(DEBUG_HTTP_ADDR) ./bin/agent

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
