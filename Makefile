# jobhound_core — personal job aggregator (collect → filter → score → storage → bot)

.PHONY: help build run test fmt vet tidy

help:
	@echo "Targets: build | run | test | fmt | vet | tidy"

build:
	go build -o bin/agent ./cmd/agent

run: build
	./bin/agent

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
