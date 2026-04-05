# JobHound — job ingest and scoring. Run: make run (agent), make run-worker (Temporal; needs JOBHOUND_TEMPORAL_ADDRESS).

.PHONY: build build-retention run run-debug run-worker test test-integration fmt vet tidy migrate-up migrate-down migrate-version

build:
	go build -o bin/agent ./cmd/agent
	go build -o bin/worker ./cmd/worker
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/retention ./cmd/retention

build-retention: build

run: build
	./bin/agent

run-debug: build
	JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:8080 ./bin/agent

run-worker: build
	./bin/worker

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
	./bin/migrate $(subst migrate-,,$@)
