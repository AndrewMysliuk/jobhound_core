# JobHound — job ingest and scoring. Full stack in Docker: make docker-up (Postgres, migrate, Redis, Temporal, agent, worker, API).
# Local binaries: make run (agent), make run-worker, etc.

COMPOSE ?= docker compose
ENV_FILE ?= .env
# Pass --env-file only when present (compose still auto-loads .env for interpolation when file exists).
COMPOSE_ENV := $(shell test -f $(ENV_FILE) && printf '%s' '--env-file $(ENV_FILE)')

.PHONY: build build-retention run run-debug run-worker test test-integration fmt vet tidy migrate-up migrate-down migrate-version \
	docker-up docker-down docker-down-volumes docker-ps docker-logs docker-migrate

build:
	go build -o bin/agent ./cmd/agent
	go build -o bin/worker ./cmd/worker
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/retention ./cmd/retention

build-retention: build

run: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/agent'

run-debug: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec env JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001 ./bin/agent'

run-worker: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/worker'

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

migrate-up migrate-down migrate-version: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/migrate $(subst migrate-,,$@)'

docker-up:
	$(COMPOSE) $(COMPOSE_ENV) build --no-cache
	$(COMPOSE) $(COMPOSE_ENV) up -d --force-recreate --pull always

docker-down:
	$(COMPOSE) $(COMPOSE_ENV) down -v --remove-orphans --rmi local

docker-down-volumes: docker-down

docker-ps:
	$(COMPOSE) $(COMPOSE_ENV) ps

docker-logs:
	$(COMPOSE) $(COMPOSE_ENV) logs -f

docker-migrate:
	$(COMPOSE) $(COMPOSE_ENV) run --rm migrate
