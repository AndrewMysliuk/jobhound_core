# JobHound — job ingest and scoring. Full stack in Docker: make docker-up (Postgres, migrate, Redis, Temporal, agent, worker, API).
# docker-up also starts bin/ipv6proxy on the host so Europe Remotely can leave the container over IPv6.
# Local binaries: make run (agent), make run-worker, etc.

COMPOSE ?= docker compose
ENV_FILE ?= .env
# Pass --env-file only when present (compose still auto-loads .env for interpolation when file exists).
COMPOSE_ENV := $(shell test -f $(ENV_FILE) && printf '%s' '--env-file $(ENV_FILE)')

.PHONY: build build-retention run run-debug run-worker eval test test-integration fmt vet lint tidy migrate-up migrate-down migrate-version \
	db-dump db-restore docker-up docker-down docker-down-volumes docker-ps docker-logs docker-migrate \
	start-ipv6-proxy stop-ipv6-proxy docs-stack companies-import

# Host CONNECT proxy. Keep the port in sync with JOBHOUND_EUROPE_REMOTELY_PROXY in docker-compose.yml.
IPV6_PROXY_ADDR ?= 127.0.0.1:18080

# Full Postgres dump for local wipe survival (gitignored under data/local/).
# Always use the compose postgres:16 client tools — host libpq 17+ injects
# SET transaction_timeout which PG16 rejects on restore.
DB_DUMP_PATH ?= data/local/postgres.dump
DB_DUMP_USER ?= jobhound
DB_DUMP_DB ?= jobhound

build:
	go build -o bin/agent ./cmd/agent
	go build -o bin/worker ./cmd/worker
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/retention ./cmd/retention
	go build -o bin/companies-import ./cmd/companies-import

build-retention: build

companies-import: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/companies-import'

run: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/agent'

run-debug: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec env JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001 ./bin/agent'

run-worker: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/worker'

eval:
	go run ./cmd/eval

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

migrate-up migrate-down migrate-version: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/migrate $(subst migrate-,,$@)'

db-dump:
	@mkdir -p "$(dir $(DB_DUMP_PATH))"
	$(COMPOSE) $(COMPOSE_ENV) exec -T postgres \
		pg_dump -U $(DB_DUMP_USER) -d $(DB_DUMP_DB) --format=custom > "$(DB_DUMP_PATH)"
	@echo "wrote $(DB_DUMP_PATH)"

db-restore: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; \
		if [ -z "$$JOBHOUND_DATABASE_URL" ]; then echo "JOBHOUND_DATABASE_URL is required (for migrate)" >&2; exit 1; fi; \
		if [ ! -f "$(DB_DUMP_PATH)" ]; then echo "dump not found: $(DB_DUMP_PATH) (run make db-dump first)" >&2; exit 1; fi; \
		set -o pipefail; \
		if $(COMPOSE) $(COMPOSE_ENV) exec -T -i postgres pg_restore --list > /dev/null < "$(DB_DUMP_PATH)"; then \
			$(COMPOSE) $(COMPOSE_ENV) exec -T -i postgres \
				pg_restore -U $(DB_DUMP_USER) -d $(DB_DUMP_DB) --clean --if-exists --no-owner --no-acl < "$(DB_DUMP_PATH)"; \
		else \
			echo "dump needs host pg_restore (pg17 archive → pg16); stripping transaction_timeout" >&2; \
			pg_restore --clean --if-exists --no-owner --no-acl --file=- "$(DB_DUMP_PATH)" \
				| sed "/^SET transaction_timeout = 0;$$/d" \
				| $(COMPOSE) $(COMPOSE_ENV) exec -T -i postgres \
					psql -U $(DB_DUMP_USER) -d $(DB_DUMP_DB) -v ON_ERROR_STOP=1; \
		fi; \
		echo "restored $(DB_DUMP_PATH)"; \
		exec ./bin/migrate up'

bin/ipv6proxy: cmd/ipv6proxy/main.go
	go build -o bin/ipv6proxy ./cmd/ipv6proxy

start-ipv6-proxy: bin/ipv6proxy
	@if curl -sf --max-time 1 "http://$(IPV6_PROXY_ADDR)/health" >/dev/null; then \
		echo "ipv6 proxy already listening on $(IPV6_PROXY_ADDR)"; \
	else \
		mkdir -p bin; \
		./bin/ipv6proxy -detach -addr "$(IPV6_PROXY_ADDR)" > bin/ipv6proxy.pid; \
		ok=; i=0; \
		while [ $$i -lt 25 ]; do \
			if curl -sf --max-time 1 "http://$(IPV6_PROXY_ADDR)/health" >/dev/null; then ok=1; break; fi; \
			i=$$((i+1)); \
			sleep 0.2; \
		done; \
		if [ -z "$$ok" ]; then \
			echo "ipv6 proxy did not start; see bin/ipv6proxy.log" >&2; \
			tail -n 20 bin/ipv6proxy.log >&2; \
			exit 1; \
		fi; \
	fi

stop-ipv6-proxy:
	@if [ -f bin/ipv6proxy.pid ]; then \
		kill $$(cat bin/ipv6proxy.pid) 2>/dev/null || true; \
		rm -f bin/ipv6proxy.pid; \
	fi

docker-up: start-ipv6-proxy
	$(COMPOSE) $(COMPOSE_ENV) build --no-cache
	$(COMPOSE) $(COMPOSE_ENV) up -d --force-recreate --pull always

docker-down: stop-ipv6-proxy
	$(COMPOSE) $(COMPOSE_ENV) down -v --remove-orphans --rmi local

docker-down-volumes: docker-down

docker-ps:
	$(COMPOSE) $(COMPOSE_ENV) ps

docker-logs:
	$(COMPOSE) $(COMPOSE_ENV) logs -f

docker-migrate:
	$(COMPOSE) $(COMPOSE_ENV) run --rm migrate

docs-stack:
	go run ./scripts/gen_stack_docs.go
