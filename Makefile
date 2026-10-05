# JobHound — job ingest and scoring. Full stack in Docker: make docker-up (Postgres, migrate, Redis, Temporal, agent, worker, API).
# docker-up also starts bin/ipv6proxy on the host so Europe Remotely can leave the container over IPv6.
# Local binaries: make run (agent), make run-worker, etc.

COMPOSE ?= docker compose
ENV_FILE ?= .env
# Pass --env-file only when present (compose still auto-loads .env for interpolation when file exists).
COMPOSE_ENV := $(shell test -f $(ENV_FILE) && printf '%s' '--env-file $(ENV_FILE)')

.PHONY: build build-retention run run-debug run-worker test test-integration fmt vet lint tidy migrate-up migrate-down migrate-version \
	docker-up docker-down docker-down-volumes docker-ps docker-logs docker-migrate \
	start-ipv6-proxy stop-ipv6-proxy

# Host CONNECT proxy. Keep the port in sync with JOBHOUND_EUROPE_REMOTELY_PROXY in docker-compose.yml.
IPV6_PROXY_ADDR ?= 127.0.0.1:18080

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

lint:
	golangci-lint run

tidy:
	go mod tidy

migrate-up migrate-down migrate-version: build
	bash -c 'if [ -f $(ENV_FILE) ]; then set -a && source $(ENV_FILE) && set +a; fi; exec ./bin/migrate $(subst migrate-,,$@)'

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
